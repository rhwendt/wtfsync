package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/collect"
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
	if got := ids(Menu(v)); !reflect.DeepEqual(got, []string{"setup", "-", "logs", "quit"}) {
		t.Fatalf("menu %v", got)
	}
}

func TestInSyncAfterSetup(t *testing.T) {
	_, _, _, nyApp := linked(t)
	v := nyApp.View()
	if !v.SetUp || v.Icon != Idle || v.Tooltip != "wtfsync — in sync" || len(v.Flavors) != 1 {
		t.Fatalf("view %+v", v)
	}
	want := []string{"open", "sync", "settings", "-", "logs", "quit"}
	if got := ids(Menu(v)); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu %v, want %v", got, want)
	}
}

func TestTickAnnouncesNewSettingsOnce(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	if err := njApp.Sync(); err != nil {
		t.Fatal(err)
	}
	nyApp.Tick()
	nyApp.Tick()
	if got := ny.notices.ids(); !reflect.DeepEqual(got, []string{"new-retail"}) {
		t.Fatalf("notices %v, want one new-retail", got)
	}
	n := ny.notices.find(t, "new-retail")
	if n.Action != "sync" || n.Body != "NJ synced new settings." {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Icon != NewSettings || v.Tooltip != "retail: NJ has newer settings" {
		t.Fatalf("view %+v", v)
	}
	if err := nyApp.Sync(); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a-nj")
	if v := nyApp.View(); v.Icon != Idle {
		t.Fatalf("after sync: %+v", v)
	}
}

func TestSyncAnnouncesMissingAddons(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.install("Plater", "## X-Curse-Project-ID: 100547")
	nj.write("Account/123#1/SavedVariables/Plater.lua", "p")
	njApp.Sync()
	if err := nyApp.Sync(); err != nil {
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
	nyApp.Sync() // same list again: no second notice
	if n := len(ny.notices.ids()); n != 1 {
		t.Fatalf("notices %v", ny.notices.ids())
	}
}

func TestGameRunningRefusesAndExplains(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Sync()
	ny.procs.running = true
	if err := nyApp.Sync(); !errors.Is(err, engine.ErrGameRunning) {
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
	nyApp.Sync()
	if v := nyApp.View(); v.Icon != Idle || v.Flavors[0].Err != "" {
		t.Fatalf("error not cleared by a good sync: %+v", v)
	}
}

func TestSyncAlreadyRunningElsewhere(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Sync()
	held, err := lock.Acquire(ny.store.Dir) // the CLI is pulling
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := nyApp.Sync(); !errors.Is(err, lock.ErrHeld) {
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
	njApp.Sync()
	ny.write(svA, "a-ny")
	ny.prompt = prompt{err: engine.ErrAborted}
	nyApp := ny.app() // picks up the cancelling prompt
	if err := nyApp.Sync(); !errors.Is(err, engine.ErrAborted) {
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
	nyApp.Sync()
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

func TestRestorePointPutsAnOlderSyncBack(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a1")
	njApp.Sync()
	nyApp.Sync()
	nj.write(svA, "a2")
	njApp.Sync()
	nyApp.Sync()
	ny.want(svA, "a2")
	points := nyApp.Main().Flavors[0].Points
	if len(points) < 3 || !points[0].Current || points[0].From != "NJ" || points[1].Current {
		t.Fatalf("points %+v", points)
	}
	older := points[1].Name // a1
	if _, err := nyApp.RestorePoint("retail", older); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a1")
	f := nyApp.Main().Flavors[0]
	if f.Points[0].Current || !f.Points[1].Current || f.Badge.Words != "1 change to sync" {
		t.Fatalf("after restore %+v", f)
	}
	// Syncing sends it to the other PC.
	if err := nyApp.Sync(); err != nil {
		t.Fatal(err)
	}
	njApp.Sync()
	nj.want(svA, "a1")
}

func TestRestorePointKeepsUnsyncedEditsInABackup(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.write(svA, "edited, not synced")
	nyApp.Refresh()
	name := nyApp.Main().Flavors[0].Points[0].Name
	undo, err := nyApp.RestorePoint("retail", name)
	if err != nil || undo == "" {
		t.Fatalf("undo %q, err %v", undo, err)
	}
	ny.want(svA, "a0")
	if err := ny.store.Restore("retail", undo, ny.path("")); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "edited, not synced")
}

func TestRestoreGonePointSaysSo(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	_, err := nyApp.RestorePoint("retail", "000099_20000101T000000Z_00000000-0000-4000-8000-000000000001.tar.gz")
	if err == nil || err.Error() != "That restore point no longer exists" || !errors.Is(err, engine.ErrNoRestorePoint) {
		t.Fatalf("err = %v", err)
	}
	ny.want(svA, "a0")
	// Nothing changed, so the flavor doesn't turn red: the page shows the error instead.
	if b := nyApp.Main().Flavors[0].Badge; b.Kind != "ok" {
		t.Fatalf("badge %+v", b)
	}
	if v := nyApp.View(); v.Icon == Attention || len(ny.notices.ids()) != 0 {
		t.Fatalf("icon %v, notices %v", v.Icon, ny.notices.ids())
	}
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
	res, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: fresh, AllFlavors: true})
	if err != nil || !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot == "" {
		t.Fatalf("empty folder: %+v, %v", res, err) // NY started it and pushed
	}
	res, err = nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: nj.sync, AllFlavors: true})
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
	if it := last[len(last)-4]; it.ID != "update" || it.Label != "Update to v0.6.0" {
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
	if err := nyApp.Sync(); !errors.Is(err, ErrBusy) {
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
		if err := nyApp.Sync(); err != nil && !errors.Is(err, ErrBusy) {
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

func TestCloseWaitsForTheRunningAction(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling")
	go func() { time.Sleep(100 * time.Millisecond); nyApp.end() }()
	start := time.Now()
	if !nyApp.Close(5 * time.Second) {
		t.Fatal("Close gave up while the action was finishing")
	}
	if d := time.Since(start); d < 100*time.Millisecond {
		t.Fatalf("Close returned after %v, before the action ended", d)
	}
	if err := nyApp.Sync(); !errors.Is(err, ErrBusy) {
		t.Fatalf("push after Close: err = %v, want ErrBusy", err)
	}
	nyApp.Tick() // must not block or run
}

func TestCloseGivesUpAfterItsWait(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling") // e.g. a conflict dialog nobody answers
	if nyApp.Close(50 * time.Millisecond) {
		t.Fatal("Close reported idle while an action was still running")
	}
	nyApp.end()
}

func TestSaveSettingsAllFlavorsOffKeepsALaterFlavorOff(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if _, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: ny.sync, Flavors: map[string]bool{"retail": true}}); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(ny.wowDir, "_classic_", "WTF", "Account"), 0o755) // installed later
	nyApp.Refresh()
	if v := nyApp.View(); len(v.Flavors) != 1 || v.Flavors[0].Name != "retail" {
		t.Fatalf("flavors %+v, want only retail", v.Flavors)
	}
	if _, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: ny.sync, AllFlavors: true}); err != nil {
		t.Fatal(err)
	}
	nyApp.Refresh()
	if v := nyApp.View(); len(v.Flavors) != 2 {
		t.Fatalf("flavors %+v, want classic and retail", v.Flavors)
	}
}

func TestFlavorActionsActOnOneFlavor(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	if err := njApp.SyncFlavor("retail"); err != nil {
		t.Fatal(err)
	}
	if err := nyApp.SyncFlavor("retail"); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a-nj")
	if err := nyApp.SyncFlavor("classic"); err == nil {
		t.Fatal("syncing a flavor that isn't enabled should fail")
	}
}

func TestFlavorActionWhileBusyIsRefused(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling")
	defer nyApp.end()
	if err := nyApp.SyncFlavor("retail"); !errors.Is(err, ErrBusy) {
		t.Fatalf("push: %v", err)
	}
	if _, err := nyApp.RestorePoint("retail", "x"); !errors.Is(err, ErrBusy) {
		t.Fatalf("restore: %v", err)
	}
}

func TestSaveSettingsWhileBusyChangesNothing(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	nyApp.begin("pulling")
	_, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: ny.sync, Flavors: map[string]bool{"retail": false}})
	nyApp.end()
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	if s, _, _ := nyApp.Settings(); !s.All() || !s.Enabled("retail") {
		t.Fatalf("settings changed anyway: %+v", s)
	}
}

func TestSaveSettingsBadWoWFolderChangesNothing(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if _, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: t.TempDir(), SyncFolder: ny.sync, Flavors: map[string]bool{"retail": false}}); err == nil {
		t.Fatal("a folder without WoW was accepted")
	}
	if s, _, _ := nyApp.Settings(); !s.All() || !s.Enabled("retail") {
		t.Fatalf("settings changed anyway: %+v", s)
	}
}

func TestOlderRefreshCantOverwriteANewerOne(t *testing.T) {
	_, _, _, nyApp := linked(t)
	older, newer := nyApp.ticket(), nyApp.ticket()
	nyApp.keep(newer, map[string]*FlavorView{"retail": {Name: "retail", Status: engine.Status{LocalChanges: 0}}})
	nyApp.keep(older, map[string]*FlavorView{"retail": {Name: "retail", Status: engine.Status{LocalChanges: 26}}})
	if v := nyApp.View(); len(v.Flavors) != 1 || v.Flavors[0].Status.LocalChanges != 0 {
		t.Fatalf("stale refresh won: %+v", v.Flavors)
	}
}

func TestFlavorNames(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	os.MkdirAll(filepath.Join(ny.wowDir, "_classic_", "WTF", "Account"), 0o755)
	if got := nyApp.FlavorNames(ny.wowDir); !reflect.DeepEqual(got, []string{"classic", "retail"}) {
		t.Fatalf("got %v", got)
	}
	if got := nyApp.FlavorNames(t.TempDir()); got == nil || len(got) != 0 {
		t.Fatalf("not a WoW folder: %#v", got)
	}
}

func TestFirstFlavorMissingAddons(t *testing.T) {
	v := View{Flavors: []FlavorView{
		{Name: "classic"},
		{Name: "retail", Status: engine.Status{MissingAddons: []engine.OtherAddons{{Machine: "NJ", Addons: []collect.MissingAddon{{Name: "Plater"}}}}}},
	}}
	if got := FirstMissing(v); got != "retail" {
		t.Fatalf("got %q", got)
	}
	if got := FirstMissing(View{}); got != "" {
		t.Fatalf("none: got %q", got)
	}
}

func TestSetAppearance(t *testing.T) {
	_, _, _, nyApp := linked(t)
	changed := 0
	nyApp.d.Changed = func() { changed++ }
	if err := nyApp.SetAppearance("dark"); err != nil {
		t.Fatal(err)
	}
	if nyApp.Main().Appearance != "dark" || changed == 0 {
		t.Fatalf("appearance %q, changed %d", nyApp.Main().Appearance, changed)
	}
	if err := nyApp.SetAppearance("purple"); err == nil {
		t.Fatal("unknown appearance accepted")
	}
	_, fresh := newPair(t)
	if err := fresh.app().SetAppearance("dark"); err == nil {
		t.Fatal("appearance saved on a PC that isn't set up")
	}
}

func TestCloudFolderNamesTheOtherPCs(t *testing.T) {
	nj, _, njApp, nyApp := linked(t)
	if got := nyApp.CloudFolder(nj.sync); !got.HasData || !reflect.DeepEqual(got.PCs, []string{"NJ"}) {
		t.Fatalf("from NY: %+v", got)
	}
	if got := njApp.CloudFolder(nj.sync); !reflect.DeepEqual(got.PCs, []string{"NY"}) {
		t.Fatalf("from NJ: %+v", got)
	}
	if got := nyApp.CloudFolder(t.TempDir()); got.HasData || got.PCs == nil || len(got.PCs) != 0 {
		t.Fatalf("empty folder: %#v", got)
	}
}

func TestCloudFolderWithDataButNoReadablePCStillHasData(t *testing.T) {
	nj, _, _, nyApp := linked(t)
	// e.g. a streaming cloud drive that hasn't downloaded NJ's record yet
	machines := filepath.Join(nj.sync, "wtfsync", "machines")
	entries, _ := os.ReadDir(machines)
	for _, e := range entries {
		os.WriteFile(filepath.Join(machines, e.Name()), []byte("not json"), 0o644)
	}
	if got := nyApp.CloudFolder(nj.sync); !got.HasData || len(got.PCs) != 0 {
		t.Fatalf("got %+v; want data and no names: the wizard must still offer Join", got)
	}
}

func TestCheckUpdateReturnsItsError(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.err = errOffline
	if err := nyApp.CheckUpdate(context.Background()); !errors.Is(err, errOffline) {
		t.Fatalf("err = %v", err)
	}
}
