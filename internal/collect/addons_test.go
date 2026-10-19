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
	want := []MissingAddon{{Name: "Bartender4", Folders: []string{"Bartender4"}}, {Name: "WeakAuras", Folders: []string{"WeakAuras"}}}
	if got := Missing(installed, other); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMissingGroupsFoldersOfOneAddon(t *testing.T) {
	other := []Addon{
		{Folder: "DBM-Core", Curse: "3358"},
		{Folder: "DBM-StatusBarTimers", Curse: "3358"}, // same project id
		{Folder: "ElvUI_Options"},                      // no id, but named after ElvUI
		{Folder: "ElvUI", Wago: "Zyz8VxGR"},
		{Folder: "Plater", Curse: "100547", Wago: "BqoN8eGb", WoWI: "24468"},
		{Folder: "Rarity", WoWI: "11025"},
		{Folder: "Odd", Curse: "12 34"}, // not a plain id: no link
	}
	want := []MissingAddon{
		{Name: "DBM-Core", Folders: []string{"DBM-Core", "DBM-StatusBarTimers"}, URL: "https://www.curseforge.com/projects/3358"},
		{Name: "ElvUI", Folders: []string{"ElvUI", "ElvUI_Options"}, URL: "https://addons.wago.io/addons/Zyz8VxGR"},
		{Name: "Odd", Folders: []string{"Odd"}},
		{Name: "Plater", Folders: []string{"Plater"}, URL: "https://www.curseforge.com/projects/100547"},
		{Name: "Rarity", Folders: []string{"Rarity"}, URL: "https://www.wowinterface.com/downloads/info11025"},
	}
	if got := Missing(nil, other); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMissingOnlyListsMissingFoldersOfAGroup(t *testing.T) {
	installed := []Addon{{Folder: "ElvUI"}}
	other := []Addon{{Folder: "ElvUI"}, {Folder: "ElvUI_Options"}}
	want := []MissingAddon{{Name: "ElvUI_Options", Folders: []string{"ElvUI_Options"}}}
	if got := Missing(installed, other); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
