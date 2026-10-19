package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicCreatesParentsAndOverwrites(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a", "b", "f.txt")
	if err := WriteAtomic(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "two" {
		t.Fatalf("read %q, %v; want two", b, err)
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}

func TestWriteJSONAndIsDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	if err := WriteJSON(p, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("got %q", b)
	}
	if !IsDir(dir) || IsDir(p) || IsDir(filepath.Join(dir, "missing")) {
		t.Fatal("IsDir wrong")
	}
}
