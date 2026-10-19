package engine

import (
	"errors"
	"path/filepath"
	"testing"
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
	want := Status{Flavor: "retail", Base: s1.String(), Newest: s2.String(), NewestBy: "NJ", Reachable: true, LocalChanges: 2}
	if st != want {
		t.Fatalf("got %+v\nwant %+v", st, want)
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
