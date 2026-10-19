package syncdir

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/collect"
)

func TestAddonListsRoundTrip(t *testing.T) {
	d, err := Init(t.TempDir(), now, h)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := d.AddonLists("retail"); err != nil || got != nil {
		t.Fatalf("empty folder: got %v, %v", got, err)
	}
	nj := AddonList{UUID: "00000000-0000-4000-8000-000000000001", Name: "NJ", Updated: now, Addons: []collect.Addon{{Folder: "Plater", Curse: "100547"}}}
	ny := AddonList{UUID: "00000000-0000-4000-8000-000000000002", Name: "NY", Updated: now}
	for _, l := range []AddonList{ny, nj} {
		if err := d.WriteAddons("retail", l); err != nil {
			t.Fatal(err)
		}
	}
	// Cloud junk next to the lists is ignored.
	dir := filepath.Join(d.Root, "retail", "addons")
	for _, junk := range []string{nj.UUID + " (1).json", nj.UUID + ".json.tmp", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, junk), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := d.AddonLists("retail")
	if err != nil {
		t.Fatal(err)
	}
	if want := []AddonList{nj, ny}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if other, _ := d.AddonLists("classic"); other != nil {
		t.Fatalf("classic: %+v", other)
	}
}
