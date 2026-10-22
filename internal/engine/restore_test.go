package engine

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/rhwendt/wtfsync/internal/archive"
)

const config = "Config.wtf" // in WTF, not synced

// restoreEnv is two snapshots from NJ, s1 {A:a1, B:b1} and s2 {A:a2}, with this PC synced to s2 and
// an unsynced Config.wtf.
func restoreEnv(t *testing.T) (v *env, s1, s2 string) {
	v = newEnv(t)
	n1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	n2 := v.remote(2, map[string]string{pathA: "a2"})
	v.synced(n2, map[string]string{pathA: "a2"})
	v.writeLocal(pathA, "a2")
	v.writeLocal(config, "cfg")
	return v, n1.String(), n2.String()
}

func TestRestoreSnapshotPutsFilesBack(t *testing.T) {
	v, s1, _ := restoreEnv(t)
	rep, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	v.want(pathA, "a1")
	v.want(pathB, "b1")
	v.want(config, "cfg")
	if want := []string{pathB, pathA}; !reflect.DeepEqual(rep.Paths, want) || rep.ID != s1 {
		t.Fatalf("report %+v, want paths %v", rep, want)
	}
	// The backup puts back what was there: A=a2 and no B.
	if err := v.eng.Store.Restore("retail", rep.Undo, v.flavor.WTFDir()); err != nil {
		t.Fatal(err)
	}
	v.want(pathA, "a2")
	if _, ok := v.local(pathB); ok {
		t.Fatal("undo left B")
	}
}

func TestRestoreSnapshotDeletesExtraFiles(t *testing.T) {
	v, _, s2 := restoreEnv(t)
	v.writeLocal(pathB, "b-local")
	if _, err := v.eng.RestoreSnapshot(v.flavor, s2, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.local(pathB); ok {
		t.Fatal("B is still there")
	}
	v.want(pathA, "a2")
	v.want(config, "cfg")
}

func TestRestoreSnapshotRecordsRestored(t *testing.T) {
	v, s1, s2 := restoreEnv(t)
	if _, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{}); err != nil {
		t.Fatal(err)
	}
	st := v.state()
	if st.Restored != s1 || st.Base != s2 {
		t.Fatalf("state %+v, want restored %s, base %s", st, s1, s2)
	}
	s, err := v.eng.Status(v.flavor)
	if err != nil {
		t.Fatal(err)
	}
	if current(s.Points) != s1 || s.LocalChanges == 0 {
		t.Fatalf("status %+v: want %s current and changes to sync", s, s1)
	}
}

func TestSyncAfterRestoreUploadsIt(t *testing.T) {
	v, s1, _ := restoreEnv(t)
	if _, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{}); err != nil {
		t.Fatal(err)
	}
	rep, err := v.eng.Push(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := v.sync.Newest("retail")
	if err != nil || n.String() != rep.Snapshot {
		t.Fatalf("newest %s, pushed %q, %v", n, rep.Snapshot, err)
	}
	data, err := v.sync.ReadSnapshot("retail", n)
	if err != nil {
		t.Fatal(err)
	}
	_, files, err := archive.Read(bytes.NewReader(data))
	if err != nil || string(files[pathA]) != "a1" || string(files[pathB]) != "b1" {
		t.Fatalf("pushed %v, %v", files, err)
	}
	if st := v.state(); st.Restored != "" || st.RestoredFiles != nil {
		t.Fatalf("state %+v still restored", st)
	}
}

func TestPullClearsRestored(t *testing.T) {
	v, s1, _ := restoreEnv(t)
	if _, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{}); err != nil {
		t.Fatal(err)
	}
	v.remote(3, map[string]string{pathA: "a3"})
	v.prompt.choices = []Choice{ChooseRemote, ChooseRemote}
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if st := v.state(); st.Restored != "" || st.RestoredFiles != nil {
		t.Fatalf("state %+v still restored", st)
	}
	v.want(pathA, "a3")
}

func TestRestoreSnapshotUnknownName(t *testing.T) {
	v, _, _ := restoreEnv(t)
	for _, name := range []string{"nope", "../../x", "000009_20261018T120000Z_" + njUUID + ".tar.gz"} {
		if _, err := v.eng.RestoreSnapshot(v.flavor, name, Options{}); !errors.Is(err, ErrNoRestorePoint) {
			t.Fatalf("%q: err %v, want ErrNoRestorePoint", name, err)
		}
	}
	v.want(pathA, "a2")
}

func TestRestoreSnapshotRefusesWhileRunning(t *testing.T) {
	v, s1, _ := restoreEnv(t)
	v.procs.running = true
	if _, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{}); !errors.Is(err, ErrGameRunning) {
		t.Fatalf("err %v, want ErrGameRunning", err)
	}
	v.want(pathA, "a2")
}

func TestRestoreSnapshotDryRun(t *testing.T) {
	v, s1, _ := restoreEnv(t)
	before := v.state()
	rep, err := v.eng.RestoreSnapshot(v.flavor, s1, Options{DryRun: true})
	if err != nil || len(rep.Paths) != 2 || rep.Undo != "" {
		t.Fatalf("report %+v, %v", rep, err)
	}
	v.want(pathA, "a2")
	if !reflect.DeepEqual(v.state(), before) {
		t.Fatal("dry run changed the state")
	}
	if sets, _ := v.eng.Store.BackupSets("retail"); slices.Contains(sets, rep.Undo) && rep.Undo != "" {
		t.Fatal("dry run made a backup")
	}
}

func TestRestoreSnapshotAlreadyCurrent(t *testing.T) {
	v, _, s2 := restoreEnv(t)
	rep, err := v.eng.RestoreSnapshot(v.flavor, s2, Options{})
	if err != nil || len(rep.Paths) != 0 || rep.Undo != "" {
		t.Fatalf("report %+v, %v; want nothing to do", rep, err)
	}
	if sets, _ := v.eng.Store.BackupSets("retail"); len(sets) != 0 {
		t.Fatalf("backups %v, want none", sets)
	}
}

func TestSyncWithNothingToUploadClearsRestored(t *testing.T) {
	v, _, s2 := restoreEnv(t)
	if _, err := v.eng.RestoreSnapshot(v.flavor, s2, Options{}); err != nil { // files already match s2
		t.Fatal(err)
	}
	if _, err := v.eng.Push(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if st := v.state(); st.Restored != "" || st.RestoredFiles != nil {
		t.Fatalf("state %+v still restored", st)
	}
}
