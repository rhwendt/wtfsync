package local

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	svA = "Account/123#1/SavedVariables/ElvUI.lua"
	svB = "Account/123#1/SavedVariables/Details.lua"
)

var t0 = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func put(t *testing.T, wtf, rel, content string) {
	t.Helper()
	p := filepath.Join(wtf, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, wtf, rel string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(wtf, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func TestBackupAndRestore(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	put(t, wtf, svA, "a-old")
	// svB doesn't exist yet: the backup records it as absent.
	id, err := s.Backup("retail", wtf, []string{svA, svB}, t0)
	if err != nil {
		t.Fatal(err)
	}
	put(t, wtf, svA, "a-new")
	put(t, wtf, svB, "b-new")
	if err := s.Restore("retail", id, wtf); err != nil {
		t.Fatal(err)
	}
	if got, _ := get(t, wtf, svA); got != "a-old" {
		t.Errorf("A = %q, want a-old", got)
	}
	if _, ok := get(t, wtf, svB); ok {
		t.Error("B should be deleted again: it was absent when backed up")
	}
}

func TestBackupSetsNewestFirstAndPruned(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	put(t, wtf, svA, "a")
	var ids []string
	for i := 0; i < 12; i++ {
		id, err := s.Backup("retail", wtf, []string{svA}, t0.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	sets, err := s.BackupSets("retail")
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 10 || sets[0] != ids[11] || sets[9] != ids[2] {
		t.Fatalf("sets = %v", sets)
	}
}

func TestRestoreUnknownOrUnsafeID(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	for _, id := range []string{"20990101T000000.000Z", "..", "../../x"} {
		if err := s.Restore("retail", id, wtf); err == nil {
			t.Errorf("Restore(%q) should fail", id)
		}
	}
}
