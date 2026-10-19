package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectPicksSavedVariablesOnly(t *testing.T) {
	wtf := t.TempDir()
	write(t, wtf, "Config.wtf", "SET gxApi \"D3D12\"")
	write(t, wtf, "Account/123#1/SavedVariables/ElvUI.lua", "elv")
	write(t, wtf, "Account/123#1/SavedVariables/ElvUI.lua.bak", "old")
	write(t, wtf, "Account/123#1/bindings-cache.wtf", "binds")
	write(t, wtf, "Account/123#1/Area 52/Thrâll/SavedVariables/WeakAuras.lua", "wa")
	write(t, wtf, "Account/123#1/Area 52/Thrâll/layout-local.txt", "layout")
	write(t, wtf, "Account/456#2/SavedVariables/Details.lua", "det")

	got, err := Collect(wtf)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{
		"Account/123#1/SavedVariables/ElvUI.lua":                    []byte("elv"),
		"Account/123#1/Area 52/Thrâll/SavedVariables/WeakAuras.lua": []byte("wa"),
		"Account/456#2/SavedVariables/Details.lua":                  []byte("det"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestCollectWithoutAccountFolder(t *testing.T) {
	got, err := Collect(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want empty, nil", got, err)
	}
}

func TestSynced(t *testing.T) {
	tests := map[string]bool{
		"Account/A/SavedVariables/X.lua":           true,
		"Account/A/R/C/SavedVariables/X.lua":       true,
		"Account/A/savedvariables/X.LUA":           true,
		"Account/A/SavedVariables/X.lua.bak":       false,
		"Config.wtf":                               false,
		"Account/A/X.lua":                          false,
		"Account/A/R/SavedVariables/X.lua":         false,
		"Other/A/SavedVariables/X.lua":             false,
		"Account/../SavedVariables/X.lua":          false,
		"Account/A/../../SavedVariables/X.lua":     false,
		"Account/./SavedVariables/X.lua":           false,
		"Account//SavedVariables/X.lua":            false,
		`Account/A\..\..\R/C/SavedVariables/X.lua`: false,
		"Account/C:/SavedVariables/X.lua":          false,
	}
	for rel, want := range tests {
		if got := Synced(rel); got != want {
			t.Errorf("Synced(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestHash(t *testing.T) {
	if got := Hash(nil); got != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("Hash(nil) = %s", got)
	}
	h := Hashes(map[string][]byte{"a": []byte("x")})
	if h["a"] != Hash([]byte("x")) || len(h) != 1 {
		t.Fatalf("Hashes = %v", h)
	}
}
