package wow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFlavorsFindsFoldersWithWTF(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"_retail_/WTF", "_classic_era_/WTF", "_ptr_", "Data", "__/WTF"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Flavors(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Flavor{
		{Name: "classic_era", Dir: filepath.Join(root, "_classic_era_")},
		{Name: "retail", Dir: filepath.Join(root, "_retail_")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestFlavorsMissingRoot(t *testing.T) {
	if _, err := Flavors(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error for missing install folder")
	}
}

func TestFlavorPaths(t *testing.T) {
	f := Flavor{Name: "retail", Dir: filepath.Join("wow", "_retail_")}
	if got, want := f.WTFDir(), filepath.Join("wow", "_retail_", "WTF"); got != want {
		t.Errorf("WTFDir = %q, want %q", got, want)
	}
	if got, want := f.AddOnsDir(), filepath.Join("wow", "_retail_", "Interface", "AddOns"); got != want {
		t.Errorf("AddOnsDir = %q, want %q", got, want)
	}
}
