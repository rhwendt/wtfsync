package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

func TestBadgeMostImportantFirst(t *testing.T) {
	ok := engine.Status{Reachable: true, Base: "b", Newest: "b"}
	for _, tt := range []struct {
		f    FlavorView
		want Badge
	}{
		{FlavorView{Name: "retail", Err: "Close WoW (retail) first", Status: engine.Status{PendingPush: true}}, Badge{"error", "!", "Close WoW (retail) first"}},
		{FlavorView{Status: engine.Status{PendingPush: true, Reachable: true}}, Badge{"error", "!", "Push pending: cloud folder unreachable"}},
		{FlavorView{Status: engine.Status{Base: "b"}}, Badge{"error", "!", "Cloud folder unreachable"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "a", Newest: "b", NewestBy: "leviathan", LocalChanges: 3}}, Badge{"newer", "↓", "leviathan has newer settings"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "b", Newest: "b", LocalChanges: 26}}, Badge{"changes", "↑ 26", "26 changes to push"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "b", Newest: "b", LocalChanges: 1}}, Badge{"changes", "↑ 1", "1 change to push"}},
		{FlavorView{Status: engine.Status{Reachable: true}}, Badge{"new", "new", "Not synced yet"}},
		{FlavorView{Status: ok}, Badge{"ok", "✓", "In sync"}},
	} {
		if got := badge(tt.f); got != tt.want {
			t.Errorf("badge(%+v) = %+v, want %+v", tt.f, got, tt.want)
		}
	}
}

func TestWhen(t *testing.T) {
	est := time.FixedZone("EST", -5*3600)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, est)
	for _, tt := range []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 10, 22, 8, 0, 0, 0, est), "today 08:00"},
		{time.Date(2026, 10, 22, 3, 51, 0, 0, time.UTC), "yesterday 22:51"}, // UTC morning is the evening before here
		{time.Date(2026, 10, 19, 2, 7, 0, 0, time.UTC), "Oct 18, 21:07"},
		{time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC), "Dec 31 2025, 07:00"},
	} {
		if got := when(tt.t, now); got != tt.want {
			t.Errorf("when(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestStatusLine(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	name := func(at time.Time) string {
		return syncdir.SnapshotName{Seq: 1, Time: at, UUID: "00000000-0000-4000-8000-000000000001"}.String()
	}
	today := name(time.Date(2026, 10, 22, 3, 51, 0, 0, time.UTC))
	oct19 := name(time.Date(2026, 10, 19, 2, 7, 0, 0, time.UTC))
	oct20 := name(time.Date(2026, 10, 20, 3, 51, 0, 0, time.UTC))
	for _, tt := range []struct {
		s    engine.Status
		want string
	}{
		{engine.Status{}, "Never synced"},
		{engine.Status{Newest: oct20, NewestBy: "leviathan"}, "Never synced · leviathan pushed Oct 20, 03:51"},
		{engine.Status{Base: today, BaseBy: "leviathan", Newest: today, NewestBy: "leviathan"}, "Last synced today 03:51 · newest from leviathan"},
		{engine.Status{Base: today, Newest: today}, "Last synced today 03:51"},
		{engine.Status{Base: oct19, BaseBy: "beehive", Newest: oct20, NewestBy: "leviathan"}, "Last synced Oct 19, 02:07 · leviathan pushed Oct 20, 03:51"},
		{engine.Status{Base: oct19}, "Last synced Oct 19, 02:07"}, // cloud folder unreachable: no newest
	} {
		if got := statusLine(tt.s, now); got != tt.want {
			t.Errorf("statusLine(%+v) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestMainShowsBackupsAndMissingAddons(t *testing.T) {
	nj, _, njApp, nyApp := linked(t)
	nj.install("Plater", "## X-Curse-Project-ID: 100547")
	nj.write(svA, "a-nj")
	njApp.Push()
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	m := nyApp.Main()
	if !m.SetUp || m.PC != "NY" || m.Version != "v0.5.0" || m.Appearance != "system" || len(m.Flavors) != 1 {
		t.Fatalf("main %+v", m)
	}
	f := m.Flavors[0]
	if f.Name != "retail" || f.Badge.Kind != "ok" {
		t.Fatalf("flavor %+v", f)
	}
	if !strings.HasPrefix(f.Line, "Last synced today ") || !strings.HasSuffix(f.Line, " · newest from NJ") {
		t.Fatalf("line %q", f.Line)
	}
	// Newest first: this pull's backup (linking made an earlier one).
	if len(f.Backups) == 0 || f.Backups[0].Reason != "before pull from NJ" || f.Backups[0].Files != 1 || !strings.HasPrefix(f.Backups[0].When, "today ") {
		t.Fatalf("backups %+v", f.Backups)
	}
	want := []MissingGroup{{PC: "NJ", Addons: []MissingAddon{{Name: "Plater", URL: "https://www.curseforge.com/projects/100547", Folders: "Plater"}}}}
	if !reflect.DeepEqual(f.Missing, want) {
		t.Fatalf("missing %+v", f.Missing)
	}
}

func TestMainDoesNotTriggerChanged(t *testing.T) {
	_, _, _, nyApp := linked(t)
	changed := 0
	nyApp.d.Changed = func() { changed++ }
	nyApp.Main()
	if changed != 0 {
		t.Fatalf("Main called Changed %d time(s): the window would reload forever", changed)
	}
}

func TestMainListsAreNeverNull(t *testing.T) {
	_, ny := newPair(t)
	b, err := json.Marshal(ny.app().Main()) // not set up: no flavors at all
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("json has null: %s", b)
	}
	_, _, _, nyApp := linked(t)
	if b, _ := json.Marshal(nyApp.Main()); strings.Contains(string(b), "null") {
		t.Fatalf("json has null: %s", b)
	}
}
