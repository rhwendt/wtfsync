package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/update"
)

func ids(m []MenuItem) []string {
	var out []string
	for _, it := range m {
		out = append(out, it.ID)
	}
	return out
}

func TestNotSetUpOffersSetupOnly(t *testing.T) {
	_, ny := newPair(t)
	a := ny.app()
	a.Refresh()
	v := a.View()
	if v.SetUp || v.Icon != Attention || v.Tooltip != "wtfsync — not set up" {
		t.Fatalf("view %+v", v)
	}
	if got := ids(Menu(v)); !reflect.DeepEqual(got, []string{"setup", "-", "quit"}) {
		t.Fatalf("menu %v", got)
	}
}

func TestInSyncAfterSetup(t *testing.T) {
	_, _, _, nyApp := linked(t)
	v := nyApp.View()
	if !v.SetUp || v.Icon != Idle || v.Tooltip != "wtfsync — in sync" || len(v.Flavors) != 1 {
		t.Fatalf("view %+v", v)
	}
	want := []string{"push", "pull", "-", "status", "restore", "settings", "-", "quit"}
	if got := ids(Menu(v)); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu %v, want %v", got, want)
	}
}

func TestTickAnnouncesNewSettingsOnce(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	if err := njApp.Push(); err != nil {
		t.Fatal(err)
	}
	nyApp.Tick()
	nyApp.Tick()
	if got := ny.notices.ids(); !reflect.DeepEqual(got, []string{"new-retail"}) {
		t.Fatalf("notices %v, want one new-retail", got)
	}
	n := ny.notices.find(t, "new-retail")
	if n.Action != "pull" || n.Body != "NJ pushed new settings." {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Icon != NewSettings || v.Tooltip != "retail: NJ has newer settings" {
		t.Fatalf("view %+v", v)
	}
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a-nj")
	if v := nyApp.View(); v.Icon != Idle {
		t.Fatalf("after pull: %+v", v)
	}
}

func TestPullAnnouncesMissingAddonsAndMenuListsThem(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.install("Plater", "## X-Curse-Project-ID: 100547")
	nj.write("Account/123#1/SavedVariables/Plater.lua", "p")
	njApp.Push()
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	if n := ny.notices.find(t, "addons"); n.Action != "addons" {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Missing != 1 || v.Icon != Attention {
		t.Fatalf("view %+v", v)
	}
	contains(t, v.Tooltip, "1 addon(s) missing here")
	found := false
	for _, it := range Menu(v) {
		found = found || (it.ID == "addons" && it.Label == "Missing addons (1)…")
	}
	if !found {
		t.Fatalf("menu %+v", Menu(v))
	}
	nyApp.Pull() // same list again: no second notice
	if n := len(ny.notices.ids()); n != 1 {
		t.Fatalf("notices %v", ny.notices.ids())
	}
}

func TestGameRunningRefusesAndExplains(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	ny.procs.running = true
	if err := nyApp.Pull(); !errors.Is(err, engine.ErrGameRunning) {
		t.Fatalf("err = %v", err)
	}
	ny.want(svA, "a0")
	if n := ny.notices.find(t, "error-retail"); n.Body != "Close WoW (retail) first" {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Icon != Attention || v.Tooltip != "retail: Close WoW (retail) first" {
		t.Fatalf("view %+v", v)
	}
	ny.procs.running = false
	nyApp.Pull()
	if v := nyApp.View(); v.Icon != Idle || v.Flavors[0].Err != "" {
		t.Fatalf("error not cleared by a good pull: %+v", v)
	}
}

func TestSyncAlreadyRunningElsewhere(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	held, err := lock.Acquire(ny.store.Dir) // the CLI is pulling
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := nyApp.Pull(); !errors.Is(err, lock.ErrHeld) {
		t.Fatalf("err = %v", err)
	}
	if n := ny.notices.find(t, "busy"); n.Body != "A sync is already running." {
		t.Fatalf("notice %+v", n)
	}
	ny.want(svA, "a0")
}

func TestCancelledConflictChangesNothing(t *testing.T) {
	nj, ny, njApp, _ := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	ny.write(svA, "a-ny")
	ny.prompt = prompt{err: engine.ErrAborted}
	nyApp := ny.app() // picks up the cancelling prompt
	if err := nyApp.Pull(); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
	ny.want(svA, "a-ny")
	if n := ny.notices.find(t, "error-retail"); n.Body != "Conflict not resolved; nothing changed" {
		t.Fatalf("notice %+v", n)
	}
}

func TestTickRetriesPendingPush(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	parked := ny.sync + ".away"
	if err := os.Rename(ny.sync, parked); err != nil { // Drive is offline
		t.Fatal(err)
	}
	ny.write(svA, "a-ny")
	nyApp.Push()
	if v := nyApp.View(); v.Icon != Attention || !v.Flavors[0].Status.PendingPush {
		t.Fatalf("offline push: %+v", v)
	}
	os.Rename(parked, ny.sync)
	nyApp.Tick()
	if st, _ := ny.store.LoadState("retail"); st.PendingPush {
		t.Fatal("pending push not retried")
	}
	if v := nyApp.View(); v.Icon != Idle {
		t.Fatalf("after retry: %+v", v)
	}
}

func TestRestoreOneFlavor(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	nyApp.Pull()
	prev, err := nyApp.RestorePreview("retail")
	if err != nil || !reflect.DeepEqual(prev.Paths, []string{svA}) || prev.Undo != "" {
		t.Fatalf("preview %+v, %v", prev, err)
	}
	ny.want(svA, "a-nj") // preview changes nothing
	rep, err := nyApp.Restore("retail")
	if err != nil || rep.Undo == "" {
		t.Fatalf("restore %+v, %v", rep, err)
	}
	ny.want(svA, "a0")
}

func TestSaveSettingsRenamesTogglesAndStartsAtLogin(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	before, _, _ := nyApp.Settings()
	res, err := nyApp.SaveSettings(SettingsInput{Name: "NY laptop", WoWDir: ny.wowDir, SyncFolder: ny.sync,
		Flavors: map[string]bool{"retail": false}, StartAtLogin: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved || res.Settings.MachineUUID != before.MachineUUID || res.Settings.MachineName != "NY laptop" || res.Settings.Flavors["retail"] {
		t.Fatalf("result %+v", res)
	}
	if !ny.auto.on || ny.auto.sets != 1 {
		t.Fatalf("autostart %+v", ny.auto)
	}
	if v := nyApp.View(); len(v.Flavors) != 0 {
		t.Fatalf("disabled flavor still shown: %+v", v.Flavors)
	}
	nyApp.SaveSettings(SettingsInput{Name: "NY laptop", WoWDir: ny.wowDir, SyncFolder: ny.sync, StartAtLogin: true})
	if ny.auto.sets != 1 {
		t.Fatal("autostart rewritten although unchanged")
	}
}

func TestSaveSettingsNewEmptyFolderStartsItNewFolderWithDataJoinsIt(t *testing.T) {
	nj, ny, _, nyApp := linked(t)
	fresh := t.TempDir()
	res, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: fresh})
	if err != nil || !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot == "" {
		t.Fatalf("empty folder: %+v, %v", res, err) // NY started it and pushed
	}
	res, err = nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: nj.sync})
	if err != nil || !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot != "" {
		t.Fatalf("folder with data: %+v, %v", res, err) // NY joined it and pulled
	}
}

func TestUpdateOfferedOnlyWhenNewer(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	for _, tt := range []struct{ latest, want string }{{"v0.5.0", ""}, {"v0.4.9", ""}, {"v0.6.0", "v0.6.0"}} {
		ny.updates.latest = update.Release{Version: tt.latest}
		nyApp.CheckUpdate(context.Background())
		if got := nyApp.View().Update; got != tt.want {
			t.Fatalf("latest %s: offered %q, want %q", tt.latest, got, tt.want)
		}
	}
	last := Menu(nyApp.View())
	if it := last[len(last)-3]; it.ID != "update" || it.Label != "Update to v0.6.0" {
		t.Fatalf("menu %+v", last)
	}
	ny.updates.err = errOffline // offline: keep quiet, keep the offer
	nyApp.CheckUpdate(context.Background())
	if nyApp.View().Update != "v0.6.0" || len(ny.notices.ids()) != 0 {
		t.Fatal("offline check changed the offer or notified")
	}
}

func TestDevBuildNeverOffersUpdates(t *testing.T) {
	_, ny := newPair(t)
	ny.updates.latest = update.Release{Version: "v9.0.0"}
	a := New(Deps{Store: ny.store, Updates: ny.updates, Version: "dev"})
	a.CheckUpdate(context.Background())
	if a.View().Update != "" {
		t.Fatal("dev build offered an update")
	}
}

func TestInstallUpdateRunsVerifiedInstaller(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.latest = update.Release{Version: "v0.6.0"}
	nyApp.CheckUpdate(context.Background())
	var ran string
	dir := t.TempDir()
	if err := nyApp.InstallUpdate(context.Background(), dir, func(p string) error { ran = p; return nil }); err != nil {
		t.Fatal(err)
	}
	if ran != filepath.Join(dir, "wtfsync-setup-v0.6.0.exe") {
		t.Fatalf("ran %q", ran)
	}
}

func TestInstallUpdateFailureRunsNothing(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.latest = update.Release{Version: "v0.6.0"}
	ny.updates.dlErr = update.ErrChecksum
	nyApp.CheckUpdate(context.Background())
	ran := false
	err := nyApp.InstallUpdate(context.Background(), t.TempDir(), func(string) error { ran = true; return nil })
	if !errors.Is(err, update.ErrChecksum) || ran {
		t.Fatalf("err %v, ran %v", err, ran)
	}
	ny.notices.find(t, "update")
	if nyApp.View().Update != "v0.6.0" {
		t.Fatal("offer withdrawn after a failed download")
	}
}

func TestActionWhileBusyIsRefused(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling")
	if err := nyApp.Push(); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
	nyApp.end()
}

func TestTicksAndActionsAtOnce(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	done := make(chan struct{})
	go func() {
		for range 20 {
			nyApp.Tick()
		}
		close(done)
	}()
	for i := range 10 {
		ny.write(svA, fmt.Sprintf("a%d", i))
		if err := nyApp.Push(); err != nil && !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
	}
	<-done
	if v := nyApp.View(); v.Busy != "" {
		t.Fatalf("left busy: %+v", v)
	}
}

func TestHasSyncData(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if !nyApp.HasSyncData(ny.sync) || nyApp.HasSyncData(t.TempDir()) {
		t.Fatal("HasSyncData")
	}
}

func TestStartsAtLoginAndVersion(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if nyApp.StartsAtLogin() || nyApp.Version() != "v0.5.0" {
		t.Fatal("defaults")
	}
	ny.auto.on = true
	if !nyApp.StartsAtLogin() {
		t.Fatal("StartsAtLogin")
	}
	if New(Deps{}).StartsAtLogin() {
		t.Fatal("no autostart setting should read as off")
	}
}
