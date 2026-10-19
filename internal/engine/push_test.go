package engine

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

func (v *env) snapshots() []syncdir.SnapshotName {
	v.t.Helper()
	all, err := v.sync.Snapshots("retail")
	if err != nil {
		v.t.Fatal(err)
	}
	return all
}

func (v *env) manifest(n syncdir.SnapshotName) (archive.Manifest, map[string][]byte) {
	v.t.Helper()
	b, err := v.sync.ReadSnapshot("retail", n)
	if err != nil {
		v.t.Fatal(err)
	}
	m, files, err := archive.Read(bytes.NewReader(b))
	if err != nil {
		v.t.Fatal(err)
	}
	return m, files
}

func TestFirstPush(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "a")
	v.writeLocal(pathC, "c")
	rep, err := v.eng.Push(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	all := v.snapshots()
	if len(all) != 1 || all[0].Seq != 1 || all[0].UUID != nyUUID || rep.Snapshot != all[0].String() || rep.Pushed != 2 {
		t.Fatalf("snapshots %v, rep %+v", all, rep)
	}
	m, files := v.manifest(all[0])
	if m.Machine.Name != "NY" || m.Base != "" || string(files[pathC]) != "c" {
		t.Fatalf("manifest %+v", m)
	}
	if st := v.state(); st.Base != all[0].String() || st.Files[pathA] != collect.Hash([]byte("a")) {
		t.Fatalf("state %+v", st)
	}
}

func TestPushNothingChanged(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "a")
	v.eng.Push(v.flavor, Options{})
	rep, err := v.eng.Push(v.flavor, Options{})
	if err != nil || !rep.UpToDate {
		t.Fatalf("rep %+v, err %v", rep, err)
	}
	if len(v.snapshots()) != 1 {
		t.Fatal("pushed an identical snapshot")
	}
}

func TestPushWhenBehindMergesFirst(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	v.synced(s1, map[string]string{pathA: "a1", pathB: "b1"})
	v.writeLocal(pathA, "a1")
	v.writeLocal(pathB, "b1")
	s2 := v.remote(2, map[string]string{pathA: "a1", pathB: "b-nj"}) // NJ changed B
	v.writeLocal(pathA, "a-ny")                                      // we changed A

	rep, err := v.eng.Push(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Merged != s2.String() {
		t.Fatalf("did not merge s2 first: %+v", rep)
	}
	newest := v.snapshots()[len(v.snapshots())-1]
	m, files := v.manifest(newest)
	if newest.Seq != 3 || m.Base != s2.String() || string(files[pathA]) != "a-ny" || string(files[pathB]) != "b-nj" {
		t.Fatalf("seq %d base %s files %q", newest.Seq, m.Base, files)
	}
	v.want(pathB, "b-nj")
}

func TestPushPendingWhenFolderUnreachable(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "a")
	folder := v.eng.Settings.SyncFolder
	v.eng.Settings.SyncFolder = filepath.Join(t.TempDir(), "drive-not-mounted")

	rep, err := v.eng.Push(v.flavor, Options{})
	if err != nil || !rep.Pending || rep.PendingReason == "" {
		t.Fatalf("rep %+v, err %v; want pending", rep, err)
	}
	if !v.state().PendingPush {
		t.Fatal("PendingPush not saved")
	}

	v.eng.Settings.SyncFolder = folder
	rep, err = v.eng.Push(v.flavor, Options{})
	if err != nil || rep.Snapshot == "" {
		t.Fatalf("retry: rep %+v, err %v", rep, err)
	}
	if v.state().PendingPush {
		t.Fatal("PendingPush not cleared")
	}
}

func TestPushPrunesTo20(t *testing.T) {
	v := newEnv(t)
	var last syncdir.SnapshotName
	for i := 1; i <= 20; i++ {
		last = v.remote(i, map[string]string{pathA: "a1"})
	}
	v.synced(last, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a2")
	if _, err := v.eng.Push(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	all := v.snapshots()
	if len(all) != 20 || all[0].Seq != 2 || all[19].Seq != 21 {
		t.Fatalf("got %d snapshots, first seq %d, last seq %d", len(all), all[0].Seq, all[len(all)-1].Seq)
	}
}

func TestPushRefusedWhileGameRunning(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "a")
	v.procs.running = true
	if _, err := v.eng.Push(v.flavor, Options{}); !errors.Is(err, ErrGameRunning) {
		t.Fatalf("err = %v", err)
	}
}

func TestPushDryRun(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "a")
	rep, err := v.eng.Push(v.flavor, Options{DryRun: true})
	if err != nil || rep.Pushed != 1 || rep.Snapshot != "" {
		t.Fatalf("rep %+v, err %v", rep, err)
	}
	if len(v.snapshots()) != 0 || v.state().Base != "" {
		t.Fatal("dry run wrote something")
	}
}
