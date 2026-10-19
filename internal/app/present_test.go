package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/setup"
)

func TestStatusRows(t *testing.T) {
	v := View{Flavors: []FlavorView{
		{Name: "classic", Status: engine.Status{Reachable: true, Base: "s1", Newest: "s1", NewestBy: "NY"}},
		{Name: "retail", Status: engine.Status{Reachable: true, Newest: "s2", NewestBy: "NJ", LocalChanges: 2}},
	}}
	want := []StatusRow{
		{Flavor: "classic", State: "in sync", Base: "s1", Newest: "s1 from NY", Local: "0 local change(s)"},
		{Flavor: "retail", State: "NJ has newer settings", Base: "never synced", Newest: "s2 from NJ", Local: "2 local change(s)"},
	}
	if got := StatusRows(v); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestAddonRows(t *testing.T) {
	v := View{Flavors: []FlavorView{{Name: "retail", Status: engine.Status{MissingAddons: []engine.OtherAddons{
		{Machine: "NJ", Addons: []collect.MissingAddon{{Name: "ElvUI", Folders: []string{"ElvUI", "ElvUI_Options"}, URL: "https://x"}}},
	}}}}}
	want := []AddonRow{{Flavor: "retail", Machine: "NJ", Name: "ElvUI", Folders: "ElvUI, ElvUI_Options", URL: "https://x"}}
	if got := AddonRows(v); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestResultLines(t *testing.T) {
	r := setup.Result{Synced: []setup.FlavorResult{
		{Flavor: "classic", Report: engine.Report{Snapshot: "s1", Pushed: 122}},
		{Flavor: "retail", Report: engine.Report{Merged: "s9", From: "NJ", Items: make([]engine.Item, 3)}},
		{Flavor: "ptr", Report: engine.Report{UpToDate: true}},
		{Flavor: "era", Err: errors.Join(engine.ErrGameRunning)},
	}}
	want := []string{"classic: uploaded 122 file(s)", "retail: got NJ's settings (3 file(s))", "ptr: nothing to do", "era: Close WoW (era) first"}
	if got := ResultLines(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if got := ResultLines(setup.Result{Rerun: true}); !reflect.DeepEqual(got, []string{"Settings saved; sync history kept."}) {
		t.Fatalf("rerun: %q", got)
	}
}
