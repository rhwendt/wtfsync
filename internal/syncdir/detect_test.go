package syncdir

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCandidates(t *testing.T) {
	home := `C:\Users\me`
	env := map[string]string{
		"OneDrive":         `C:\Users\me\OneDrive`,
		"OneDriveConsumer": `C:\Users\me\OneDrive`, // same as OneDrive: listed once
		"USERPROFILE":      home,
	}
	exists := map[string]bool{
		`C:\Users\me\OneDrive`:             true,
		filepath.Join(home, "Dropbox"):     true,
		filepath.Join(home, "iCloudDrive"): false,
		`G:\My Drive`:                      true,
	}
	got := Candidates(func(k string) string { return env[k] }, func(p string) bool { return exists[p] })
	want := []string{`C:\Users\me\OneDrive`, filepath.Join(home, "Dropbox"), `G:\My Drive`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCandidatesNone(t *testing.T) {
	got := Candidates(func(string) string { return "" }, func(string) bool { return false })
	if len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
