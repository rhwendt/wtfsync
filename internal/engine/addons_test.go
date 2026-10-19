package engine

import (
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// njAddons records NJ's installed addons in the sync folder, as NJ's push would.
func (v *env) njAddons(addons ...collect.Addon) {
	v.t.Helper()
	if err := v.sync.WriteAddons("retail", syncdir.AddonList{UUID: njUUID, Name: "NJ", Updated: v.clock, Addons: addons}); err != nil {
		v.t.Fatal(err)
	}
}

func (v *env) install(folder string) {
	v.t.Helper()
	v.writeLocal("../Interface/AddOns/"+folder+"/"+folder+".toc", "## Version: 1\n## X-Curse-Project-ID: 42")
}

var wantWeakAuras = []OtherAddons{{Machine: "NJ", Addons: []collect.MissingAddon{{Name: "WeakAuras", Folders: []string{"WeakAuras"}}}}}

func TestPullReportsMissingAddons(t *testing.T) {
	v := newEnv(t)
	v.install("ElvUI")
	v.njAddons(collect.Addon{Folder: "ElvUI"}, collect.Addon{Folder: "WeakAuras"})
	v.remote(1, map[string]string{pathA: "a"})
	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.MissingAddons, wantWeakAuras) {
		t.Fatalf("missing = %+v", rep.MissingAddons)
	}
}

func TestPullUpToDateStillReportsMissingAddons(t *testing.T) {
	v := newEnv(t)
	v.njAddons(collect.Addon{Folder: "WeakAuras"})
	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil || !rep.UpToDate {
		t.Fatalf("rep %+v, err %v", rep, err)
	}
	if !reflect.DeepEqual(rep.MissingAddons, wantWeakAuras) {
		t.Fatalf("missing = %+v", rep.MissingAddons)
	}
}

func TestStatusReportsMissingAddons(t *testing.T) {
	v := newEnv(t)
	v.njAddons(collect.Addon{Folder: "WeakAuras"})
	st, err := v.eng.Status(v.flavor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st.MissingAddons, wantWeakAuras) {
		t.Fatalf("missing = %+v", st.MissingAddons)
	}
	v.install("WeakAuras")
	if st, _ := v.eng.Status(v.flavor); st.MissingAddons != nil {
		t.Fatalf("after install: %+v", st.MissingAddons)
	}
}

func TestPushRecordsInstalledAddons(t *testing.T) {
	v := newEnv(t)
	v.install("Plater")
	v.writeLocal(pathA, "a")
	for range 2 { // the second push has nothing new but still refreshes the list
		if _, err := v.eng.Push(v.flavor, Options{}); err != nil {
			t.Fatal(err)
		}
	}
	lists, err := v.sync.AddonLists("retail")
	if err != nil {
		t.Fatal(err)
	}
	want := []collect.Addon{{Folder: "Plater", Version: "1", Curse: "42"}}
	if len(lists) != 1 || lists[0].UUID != nyUUID || lists[0].Name != "NY" || !reflect.DeepEqual(lists[0].Addons, want) {
		t.Fatalf("lists = %+v", lists)
	}
}

func TestPushDryRunWritesNoAddonList(t *testing.T) {
	v := newEnv(t)
	v.install("Plater")
	if _, err := v.eng.Push(v.flavor, Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if lists, _ := v.sync.AddonLists("retail"); lists != nil {
		t.Fatalf("lists = %+v", lists)
	}
}
