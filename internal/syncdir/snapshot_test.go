package syncdir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	uuidA = "00000000-0000-4000-8000-00000000000a"
	uuidB = "00000000-0000-4000-8000-00000000000b"
)

func name(seq int, minute int, uuid string) SnapshotName {
	return SnapshotName{Seq: seq, Time: now.Add(time.Duration(minute) * time.Minute), UUID: uuid}
}

func TestNameRoundTrip(t *testing.T) {
	n := name(42, 0, uuidA)
	if got, want := n.String(), "000042_20261018T120000Z_"+uuidA+".tar.gz"; got != want {
		t.Fatalf("String = %s, want %s", got, want)
	}
	back, ok := ParseName(n.String())
	if !ok || back.Seq != 42 || !back.Time.Equal(n.Time) || back.UUID != uuidA {
		t.Fatalf("ParseName = %+v, %v", back, ok)
	}
}

func TestParseNameRejectsJunk(t *testing.T) {
	for _, s := range []string{
		"000001_x.tar.gz",
		"file (1).tar.gz",
		"000001_20261018T120000Z_" + uuidA + " (1).tar.gz",
		"000001_20261018T120000Z_" + uuidA + ".tar.gz.tmp",
		"1_20261018T120000Z_" + uuidA + ".tar.gz",
	} {
		if _, ok := ParseName(s); ok {
			t.Errorf("ParseName(%q) accepted", s)
		}
	}
}

func TestOrderingIgnoresClockSkew(t *testing.T) {
	d, _ := Init(t.TempDir(), now, h)
	// seq 2 was written by a PC whose clock is an hour behind: it is still newest.
	for _, n := range []SnapshotName{name(1, 0, uuidA), name(2, -60, uuidB)} {
		if err := d.WriteSnapshot("retail", n, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := d.Newest("retail")
	if err != nil || !ok || got.Seq != 2 {
		t.Fatalf("Newest = %+v, %v, %v; want seq 2", got, ok, err)
	}
}

func TestSameSeqTieBreak(t *testing.T) {
	if !name(3, 0, uuidB).Before(name(3, 1, uuidA)) {
		t.Error("same seq: later time should sort after")
	}
	if !name(3, 0, uuidA).Before(name(3, 0, uuidB)) {
		t.Error("same seq and time: UUID breaks the tie")
	}
}

func TestSnapshotsIgnoresJunkAndEmptyFlavor(t *testing.T) {
	d, _ := Init(t.TempDir(), now, h)
	if all, err := d.Snapshots("retail"); err != nil || len(all) != 0 {
		t.Fatalf("empty flavor: %v, %v", all, err)
	}
	n := name(1, 0, uuidA)
	d.WriteSnapshot("retail", n, []byte("x"))
	dir := filepath.Join(d.Root, "retail", "snapshots")
	os.WriteFile(filepath.Join(dir, "000009_20261018T120000Z_"+uuidA+" (1).tar.gz"), []byte("copy"), 0o644)
	os.WriteFile(filepath.Join(dir, "000009_20261018T120000Z_"+uuidA+".tar.gz.tmp"), []byte("tmp"), 0o644)
	all, err := d.Snapshots("retail")
	if err != nil || len(all) != 1 || all[0].String() != n.String() {
		t.Fatalf("Snapshots = %v, %v", all, err)
	}
}

func TestWriteSnapshotIsImmutable(t *testing.T) {
	d, _ := Init(t.TempDir(), now, h)
	n := name(1, 0, uuidA)
	if err := d.WriteSnapshot("retail", n, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteSnapshot("retail", n, []byte("two")); err == nil {
		t.Fatal("overwriting a snapshot should fail")
	}
	b, err := d.ReadSnapshot("retail", n)
	if err != nil || string(b) != "one" {
		t.Fatalf("ReadSnapshot = %q, %v", b, err)
	}
}

type stuck struct{}

func (stuck) ReadFile(string) ([]byte, error) { return nil, ErrNotHydrated }

func TestReadSnapshotUsesHydrator(t *testing.T) {
	d, _ := Init(t.TempDir(), now, h)
	n := name(1, 0, uuidA)
	d.WriteSnapshot("retail", n, []byte("x"))
	d.Hydrator = stuck{}
	if _, err := d.ReadSnapshot("retail", n); !errors.Is(err, ErrNotHydrated) {
		t.Fatalf("err = %v, want ErrNotHydrated", err)
	}
}

func TestPruneKeepsNewestAndProtected(t *testing.T) {
	d, _ := Init(t.TempDir(), now, h)
	var ns []SnapshotName
	for i := 1; i <= 4; i++ {
		n := name(i, i, uuidA)
		ns = append(ns, n)
		d.WriteSnapshot("retail", n, []byte("x"))
	}
	if err := d.Prune("retail", 2, map[string]bool{ns[0].String(): true}); err != nil {
		t.Fatal(err)
	}
	all, _ := d.Snapshots("retail")
	var seqs []int
	for _, n := range all {
		seqs = append(seqs, n.Seq)
	}
	if len(seqs) != 3 || seqs[0] != 1 || seqs[1] != 3 || seqs[2] != 4 {
		t.Fatalf("after prune seqs = %v, want [1 3 4]", seqs)
	}
}
