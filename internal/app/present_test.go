package app

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/setup"
)

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

// The pages loop over these; JSON null (a nil slice) would throw and leave the window blank.
func TestEmptyRowsAreEmptyLists(t *testing.T) {
	for name, v := range map[string]any{
		"ResultLines": ResultLines(setup.Result{}),
	} {
		if b, _ := json.Marshal(v); string(b) != "[]" {
			t.Errorf("%s = %s, want []", name, b)
		}
	}
}
