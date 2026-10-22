package app

import (
	"strings"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
)

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}

func TestMenuDisablesSyncWhileBusy(t *testing.T) {
	v := View{SetUp: true, Busy: "pulling"}
	summarize(&v)
	if v.Icon != Syncing || v.Tooltip != "wtfsync — pulling…" {
		t.Fatalf("view %+v", v)
	}
	for _, it := range Menu(v) {
		want := it.ID == "open" || it.ID == "logs" || it.ID == "quit"
		if it.ID != "-" && it.Enabled != want {
			t.Errorf("%s enabled = %v while busy", it.ID, it.Enabled)
		}
	}
}

func TestTooltipFitsWindowsLimit(t *testing.T) {
	v := View{SetUp: true, Flavors: []FlavorView{
		{Name: "retail", Status: engine.Status{Reachable: true}, Err: string(make([]rune, 300))},
		{Name: "classic", Status: engine.Status{PendingPush: true, Reachable: true}},
	}}
	summarize(&v)
	if n := len([]rune(v.Tooltip)); n > tooltipMax {
		t.Fatalf("tooltip has %d runes", n)
	}
	contains(t, v.Tooltip, "…")
}
