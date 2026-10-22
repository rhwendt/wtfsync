package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/local"
)

func TestStatus(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a2")
	v.writeLocal(pathB, "b")
	s2 := v.remote(2, map[string]string{pathA: "a1"})

	st, err := v.eng.Status(v.flavor)
	if err != nil {
		t.Fatal(err)
	}
	want := Status{Flavor: "retail", Base: s1.String(), BaseBy: "NJ", Newest: s2.String(), NewestBy: "NJ", Reachable: true, LocalChanges: 2,
		Points: []RestorePoint{{Name: s2.String(), Time: s2.Time, From: "NJ"}, {Name: s1.String(), Time: s1.Time, From: "NJ"}}}
	if !reflect.DeepEqual(st, want) {
		t.Fatalf("got %+v\nwant %+v", st, want)
	}
}

func points(t *testing.T, v *env) []RestorePoint {
	t.Helper()
	st, err := v.eng.Status(v.flavor)
	if err != nil {
		t.Fatal(err)
	}
	return st.Points
}

func current(ps []RestorePoint) string {
	for _, p := range ps {
		if p.Current {
			return p.Name
		}
	}
	return ""
}

func TestStatusPointsNewestFirstAndCurrentBase(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	s2 := v.remote(2, map[string]string{pathA: "a2"})
	v.synced(s2, map[string]string{pathA: "a2"})
	v.writeLocal(pathA, "a2")
	ps := points(t, v)
	want := []RestorePoint{{Name: s2.String(), Time: s2.Time, From: "NJ", Current: true}, {Name: s1.String(), Time: s1.Time, From: "NJ"}}
	if !reflect.DeepEqual(ps, want) {
		t.Fatalf("got %+v\nwant %+v", ps, want)
	}
}

func TestStatusPointsNoneCurrentWithLocalChanges(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "edited")
	if c := current(points(t, v)); c != "" {
		t.Fatalf("current = %q, want none", c)
	}
}

func TestStatusPointsUnknownPC(t *testing.T) {
	v := newEnv(t)
	v.remote(1, map[string]string{pathA: "a1"})
	if err := os.Remove(filepath.Join(v.sync.Root, "machines", njUUID+".json")); err != nil {
		t.Fatal(err)
	}
	if ps := points(t, v); len(ps) != 1 || ps[0].From != njUUID[:8] {
		t.Fatalf("got %+v, want From %q", ps, njUUID[:8])
	}
}

func TestStatusPointsRestored(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	s2 := v.remote(2, map[string]string{pathA: "a2"})
	v.writeLocal(pathA, "a1")
	st := local.FlavorState{Base: s2.String(), Files: map[string]string{pathA: collect.Hash([]byte("a2"))},
		Restored: s1.String(), RestoredFiles: map[string]string{pathA: collect.Hash([]byte("a1"))}}
	if err := v.eng.Store.SaveState("retail", st); err != nil {
		t.Fatal(err)
	}
	if c := current(points(t, v)); c != s1.String() {
		t.Fatalf("current = %q, want %q", c, s1)
	}
	v.writeLocal(pathA, "edited")
	if c := current(points(t, v)); c != "" {
		t.Fatalf("after an edit current = %q, want none", c)
	}
}

func TestStatusUnreachable(t *testing.T) {
	v := newEnv(t)
	v.eng.Settings.SyncFolder = filepath.Join(t.TempDir(), "gone")
	st, err := v.eng.Status(v.flavor)
	if err != nil || st.Reachable {
		t.Fatalf("got %+v, %v", st, err)
	}
}

func TestRestoreNewestBackup(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a1")
	v.remote(2, map[string]string{pathA: "a2"})
	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := v.eng.Restore(v.flavor, "", Options{})
	if err != nil || r.ID != rep.Backup {
		t.Fatalf("id %q, err %v; want %q", r.ID, err, rep.Backup)
	}
	v.want(pathA, "a1")
}

func TestRestoreNoBackups(t *testing.T) {
	v := newEnv(t)
	if _, err := v.eng.Restore(v.flavor, "", Options{}); err == nil {
		t.Fatal("want error with no backups")
	}
}

func TestRestoreRefusedWhileGameRunning(t *testing.T) {
	v := newEnv(t)
	v.procs.running = true
	if _, err := v.eng.Restore(v.flavor, "", Options{}); !errors.Is(err, ErrGameRunning) {
		t.Fatalf("err = %v", err)
	}
}

func TestBackupsSayWhyTheyWereMade(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a1")
	v.remote(2, map[string]string{pathA: "a2"})
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.eng.Restore(v.flavor, "", Options{}); err != nil {
		t.Fatal(err)
	}
	got, err := v.eng.Store.Backups("retail")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Reason != local.ReasonRestore || got[1].Reason != local.ReasonPull || got[1].From != "NJ" {
		t.Fatalf("backups %+v", got)
	}
}

func TestRestoreOldestOfAFullSetOfBackups(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "oldest")
	oldest, err := v.eng.Store.Backup("retail", v.flavor.WTFDir(), []string{pathA}, v.eng.Now(), local.Why{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 9 { // fill the store to its 10 kept sets
		v.writeLocal(pathA, fmt.Sprint("later ", i))
		if _, err := v.eng.Store.Backup("retail", v.flavor.WTFDir(), []string{pathA}, v.eng.Now(), local.Why{}); err != nil {
			t.Fatal(err)
		}
	}
	v.writeLocal(pathA, "now")
	if _, err := v.eng.Restore(v.flavor, oldest, Options{}); err != nil {
		t.Fatalf("restoring the oldest kept backup: %v", err)
	}
	v.want(pathA, "oldest")
	sets, _ := v.eng.Store.Backups("retail")
	if len(sets) != 10 || sets[0].Reason != local.ReasonRestore {
		t.Fatalf("after restore: %d sets, newest %+v", len(sets), sets[0])
	}
}
