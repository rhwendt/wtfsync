package syncdir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	h   = TimeoutHydrator{Timeout: 5 * time.Second}
	now = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
)

func TestInitThenOpen(t *testing.T) {
	folder := t.TempDir()
	d, err := Init(folder, now, h)
	if err != nil {
		t.Fatal(err)
	}
	if d.Root != filepath.Join(folder, "wtfsync") {
		t.Fatalf("Root = %s", d.Root)
	}
	if _, err := os.Stat(filepath.Join(folder, "wtfsync", "wtfsync.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(folder, h); err != nil {
		t.Fatal(err)
	}
}

func TestInitRefusesExisting(t *testing.T) {
	folder := t.TempDir()
	if _, err := Init(folder, now, h); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(folder, now, h); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("err = %v, want ErrAlreadyInitialized", err)
	}
}

func TestOpenNotInitialized(t *testing.T) {
	if _, err := Open(t.TempDir(), h); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("err = %v, want ErrNotInitialized", err)
	}
}

func TestUnreachableFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "G-drive-not-mounted")
	if _, err := Open(missing, h); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("Open err = %v, want ErrUnreachable", err)
	}
	if _, err := Init(missing, now, h); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("Init err = %v, want ErrUnreachable", err)
	}
}

func TestOpenRejectsUnknownFormat(t *testing.T) {
	folder := t.TempDir()
	p := filepath.Join(folder, "wtfsync", "wtfsync.json")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(`{"format":2,"encryption":"none"}`), 0o644)
	if _, err := Open(folder, h); err == nil {
		t.Fatal("want error for format 2")
	}
}

func TestMachineRecords(t *testing.T) {
	d, err := Init(t.TempDir(), now, h)
	if err != nil {
		t.Fatal(err)
	}
	r := MachineRecord{UUID: "00000000-0000-4000-8000-000000000001", Name: "NJ", LastSeen: now}
	if err := d.WriteMachine(r); err != nil {
		t.Fatal(err)
	}
	got, err := d.Machine(r.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "NJ" || got.UUID != r.UUID || !got.LastSeen.Equal(now) {
		t.Fatalf("got %+v", got)
	}
}
