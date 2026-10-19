package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/collect"
)

const (
	pathA = "Account/123#1/SavedVariables/ElvUI.lua"
	pathC = "Account/123#1/Area 52/Thrâll/SavedVariables/WeakAuras.lua"
)

func TestRoundTrip(t *testing.T) {
	m := Manifest{
		Machine: Machine{UUID: "00000000-0000-4000-8000-000000000001", Name: "NJ"},
		Created: time.Date(2026, 10, 18, 15, 30, 0, 0, time.UTC),
		Flavor:  "retail",
		Seq:     3,
		Base:    "000002_20261017T220000Z_00000000-0000-4000-8000-000000000002.tar.gz",
		Addons:  []collect.Addon{{Folder: "ElvUI", Version: "13.74"}},
	}
	files := map[string][]byte{pathA: []byte("elv"), pathC: []byte("wa")}
	var buf bytes.Buffer
	if err := Write(&buf, m, files); err != nil {
		t.Fatal(err)
	}
	got, gotFiles, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	m.Format = Format
	m.Files = collect.Hashes(files)
	if !got.Created.Equal(m.Created) {
		t.Fatalf("Created = %v, want %v", got.Created, m.Created)
	}
	got.Created = m.Created
	if !reflect.DeepEqual(got, m) {
		t.Fatalf("manifest\n got %+v\nwant %+v", got, m)
	}
	if !reflect.DeepEqual(gotFiles, files) {
		t.Fatalf("files = %q, want %q", gotFiles, files)
	}
}

// raw builds a gzip tar from (name, body) entries in order, for crafting bad snapshots.
func raw(t *testing.T, entries ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e[0], Mode: 0o644, Size: int64(len(e[1])), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func manifest(files map[string]string) string {
	s := `{"format":1,"files":{`
	first := true
	for p, h := range files {
		if !first {
			s += ","
		}
		s += fmt.Sprintf("%q:%q", p, h)
		first = false
	}
	return s + "}}"
}

func TestReadRejectsCorrupt(t *testing.T) {
	var good bytes.Buffer
	if err := Write(&good, Manifest{}, map[string][]byte{pathA: bytes.Repeat([]byte("x"), 4096)}); err != nil {
		t.Fatal(err)
	}
	hx := collect.Hash([]byte("x"))
	tests := map[string][]byte{
		"not gzip":           []byte("hello"),
		"truncated":          good.Bytes()[:good.Len()/2],
		"manifest not first": raw(t, [2]string{pathA, "x"}, [2]string{"manifest.json", manifest(map[string]string{pathA: hx})}),
		"unsupported format": raw(t, [2]string{"manifest.json", `{"format":2,"files":{}}`}),
		"hash mismatch":      raw(t, [2]string{"manifest.json", manifest(map[string]string{pathA: hx})}, [2]string{pathA, "y"}),
		"unsafe path":        raw(t, [2]string{"manifest.json", manifest(map[string]string{"../evil.lua": hx})}, [2]string{"../evil.lua", "x"}),
		"file missing":       raw(t, [2]string{"manifest.json", manifest(map[string]string{pathA: hx})}),
		"unlisted file":      raw(t, [2]string{"manifest.json", manifest(nil)}, [2]string{pathA, "x"}),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Read(bytes.NewReader(data)); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("err = %v, want ErrCorrupt", err)
			}
		})
	}
}
