// Package archive builds and reads snapshot files: a gzip tar holding manifest.json first, then each
// SavedVariables file at its WTF-relative path.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/rhwendt/wtfsync/internal/collect"
)

// Format is the snapshot format version this build reads and writes.
const Format = 1

const manifestName = "manifest.json"

var ErrCorrupt = errors.New("corrupt snapshot")

type Machine struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type Manifest struct {
	Format  int               `json:"format"`
	Machine Machine           `json:"machine"`
	Created time.Time         `json:"created"`
	Flavor  string            `json:"flavor"`
	Seq     int               `json:"seq"`
	Base    string            `json:"base"`  // snapshot the writer was synced with; "" for its first push
	Files   map[string]string `json:"files"` // WTF-relative path → collect.Hash
	Addons  []collect.Addon   `json:"addons"`
}

// Write writes a snapshot of files. It sets m.Format and fills m.Files from files.
func Write(w io.Writer, m Manifest, files map[string][]byte) error {
	m.Format = Format
	m.Files = collect.Hashes(files)
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := writeEntry(tw, manifestName, mj, m.Created); err != nil {
		return err
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if err := writeEntry(tw, p, files[p], m.Created); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func writeEntry(tw *tar.Writer, name string, body []byte, mod time.Time) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), ModTime: mod, Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}

func corrupt(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

// Read parses a snapshot and checks it: manifest first, known format, only safe synced paths, and
// every listed file present with the listed hash. Any problem is reported as ErrCorrupt.
func Read(r io.Reader) (Manifest, map[string][]byte, error) {
	var m Manifest
	gz, err := gzip.NewReader(r)
	if err != nil {
		return m, nil, corrupt("%v", err)
	}
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil || hdr.Name != manifestName {
		return m, nil, corrupt("manifest.json is not the first entry")
	}
	mj, err := io.ReadAll(tr)
	if err != nil {
		return m, nil, corrupt("%v", err)
	}
	if err := json.Unmarshal(mj, &m); err != nil {
		return m, nil, corrupt("manifest: %v", err)
	}
	if m.Format != Format {
		return m, nil, corrupt("unsupported format %d", m.Format)
	}
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, nil, corrupt("%v", err)
		}
		if !collect.Synced(hdr.Name) {
			return m, nil, corrupt("unexpected path %q", hdr.Name)
		}
		want, ok := m.Files[hdr.Name]
		if !ok {
			return m, nil, corrupt("%s is not in the manifest", hdr.Name)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return m, nil, corrupt("%v", err)
		}
		if collect.Hash(b) != want {
			return m, nil, corrupt("%s does not match its hash", hdr.Name)
		}
		files[hdr.Name] = b
	}
	if _, err := io.Copy(io.Discard, gz); err != nil { // verifies the gzip checksum
		return m, nil, corrupt("%v", err)
	}
	if len(files) != len(m.Files) {
		return m, nil, corrupt("%d file(s) listed in the manifest are missing", len(m.Files)-len(files))
	}
	return m, files, nil
}
