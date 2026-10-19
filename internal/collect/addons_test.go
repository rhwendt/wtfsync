package collect

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAddonsReadsTOC(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ElvUI/ElvUI_Mainline.toc", "\ufeff## Interface: 110200\n## Title: ElvUI\n## Version: 13.74\n## X-Curse-Project-ID: 1111\n## X-WoWI-ID: 2222\n")
	write(t, dir, "WeakAuras/WeakAuras.toc", "## Version: 5.19\n##X-Wago-ID: abcd\n")
	write(t, dir, "WeakAuras/WeakAuras_Vanilla.toc", "## Version: wrong-one\n")
	write(t, dir, "NoToc/readme.txt", "hi")
	write(t, dir, "stray.txt", "x")

	got, err := Addons(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Addon{
		{Folder: "ElvUI", Version: "13.74", Curse: "1111", WoWI: "2222"},
		{Folder: "WeakAuras", Version: "5.19", Wago: "abcd"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestAddonsMissingFolder(t *testing.T) {
	got, err := Addons(filepath.Join(t.TempDir(), "nope"))
	if err != nil || got != nil {
		t.Fatalf("got %v, %v; want nil, nil", got, err)
	}
}

func TestMissing(t *testing.T) {
	installed := []Addon{{Folder: "ElvUI"}, {Folder: "details"}}
	other := []Addon{{Folder: "WeakAuras"}, {Folder: "Details"}, {Folder: "Bartender4"}, {Folder: "ElvUI"}}
	if got, want := Missing(installed, other), []string{"Bartender4", "WeakAuras"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
