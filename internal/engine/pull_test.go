package engine

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

func TestPullNothingToPull(t *testing.T) {
	v := newEnv(t)
	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil || !rep.UpToDate {
		t.Fatalf("rep %+v, err %v; want up to date", rep, err)
	}
	s1 := v.remote(1, map[string]string{pathA: "a"})
	v.synced(s1, map[string]string{pathA: "a"})
	if rep, err := v.eng.Pull(v.flavor, Options{}); err != nil || !rep.UpToDate {
		t.Fatalf("already at newest: rep %+v, err %v", rep, err)
	}
}

func TestPullTakesRemoteKeepsLocalAndBacksUp(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	v.synced(s1, map[string]string{pathA: "a1", pathB: "b1"})
	v.writeLocal(pathA, "a1")
	v.writeLocal(pathB, "b2")                                      // edited here
	s2 := v.remote(2, map[string]string{pathA: "a2", pathB: "b1"}) // edited on NJ

	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	v.want(pathA, "a2")
	v.want(pathB, "b2")
	if rep.Merged != s2.String() || rep.From != "NJ" {
		t.Fatalf("rep = %+v", rep)
	}
	if want := []Item{{pathB, KeepLocal}, {pathA, TakeRemote}}; !reflect.DeepEqual(rep.Items, want) { // sorted by path
		t.Fatalf("items = %v, want %v", rep.Items, want)
	}
	if st := v.state(); st.Base != s2.String() || st.Files[pathA] != collect.Hash([]byte("a2")) {
		t.Fatalf("state = %+v", st)
	}
	if err := v.eng.Store.Restore("retail", rep.Backup, v.flavor.WTFDir()); err != nil {
		t.Fatal(err)
	}
	v.want(pathA, "a1")
}

func TestPullPropagatesDeletion(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a", pathC: "c"})
	v.synced(s1, map[string]string{pathA: "a", pathC: "c"})
	v.writeLocal(pathA, "a")
	v.writeLocal(pathC, "c")
	v.remote(2, map[string]string{pathA: "a"}) // NJ deleted the character

	rep, err := v.eng.Pull(v.flavor, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.local(pathC); ok {
		t.Fatal("C should be deleted")
	}
	v.eng.Store.Restore("retail", rep.Backup, v.flavor.WTFDir())
	v.want(pathC, "c")
}

func TestPullConflictAsks(t *testing.T) {
	for _, tt := range []struct {
		choice Choice
		want   string
	}{{ChooseLocal, "a-ny"}, {ChooseRemote, "a-nj"}} {
		v := newEnv(t)
		s1 := v.remote(1, map[string]string{pathA: "a1"})
		v.synced(s1, map[string]string{pathA: "a1"})
		v.writeLocal(pathA, "a-ny")
		v.remote(2, map[string]string{pathA: "a-nj"})
		v.prompt.choices = []Choice{tt.choice}

		if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
			t.Fatal(err)
		}
		v.want(pathA, tt.want)
		if !reflect.DeepEqual(v.prompt.asked, []string{pathA}) {
			t.Fatalf("asked = %v", v.prompt.asked)
		}
	}
}

func TestPullFirstLinkAsksOnce(t *testing.T) {
	for _, tt := range []struct {
		choice FirstLinkChoice
		wantA  string
	}{{AllRemote, "theirs"}, {AllLocal, "mine"}} {
		v := newEnv(t)
		v.writeLocal(pathA, "mine")
		v.remote(1, map[string]string{pathA: "theirs", pathB: "b"})
		v.prompt.firstLink = []FirstLinkChoice{tt.choice}

		if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
			t.Fatal(err)
		}
		v.want(pathA, tt.wantA)
		v.want(pathB, "b") // only on NJ: always taken
		if len(v.prompt.asked) != 0 {
			t.Fatalf("per-file prompts: %v", v.prompt.asked)
		}
	}
}

func TestPullFirstLinkPerFile(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "mine")
	v.remote(1, map[string]string{pathA: "theirs"})
	v.prompt.firstLink = []FirstLinkChoice{PerFile}
	v.prompt.choices = []Choice{ChooseLocal}
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	v.want(pathA, "mine")
}

func TestPullAbortChangesNothing(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	v.synced(s1, map[string]string{pathA: "a1", pathB: "b1"})
	v.writeLocal(pathA, "a-ny")
	v.writeLocal(pathB, "b1")
	v.remote(2, map[string]string{pathA: "a-nj", pathB: "b2"})
	// no scripted answers: the conflict prompt aborts

	_, err := v.eng.Pull(v.flavor, Options{})
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	v.want(pathA, "a-ny")
	v.want(pathB, "b1") // the non-conflicting take was not applied either
	if v.state().Base != s1.String() {
		t.Fatal("base moved after abort")
	}
}

func TestPullRefusedWhileGameRunning(t *testing.T) {
	v := newEnv(t)
	v.remote(1, map[string]string{pathA: "a"})
	v.procs.running = true
	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, ErrGameRunning) {
		t.Fatalf("err = %v, want ErrGameRunning", err)
	}
	if _, ok := v.local(pathA); ok {
		t.Fatal("wrote while game running")
	}
}

func TestPullDryRun(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a-ny")
	v.remote(2, map[string]string{pathA: "a-nj"})

	rep, err := v.eng.Pull(v.flavor, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Items, []Item{{pathA, Conflict}}) || len(v.prompt.asked) != 0 {
		t.Fatalf("items %v, asked %v", rep.Items, v.prompt.asked)
	}
	v.want(pathA, "a-ny")
	if v.state().Base != s1.String() {
		t.Fatal("dry run moved base")
	}
}

func TestPullCorruptSnapshotTouchesNothing(t *testing.T) {
	v := newEnv(t)
	v.writeLocal(pathA, "mine")
	bad := syncdir.SnapshotName{Seq: 1, Time: time.Date(2026, 10, 18, 13, 0, 0, 0, time.UTC), UUID: njUUID}
	v.sync.WriteSnapshot("retail", bad, []byte("not a snapshot"))

	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, archive.ErrCorrupt) {
		t.Fatalf("err = %v, want ErrCorrupt", err)
	}
	v.want(pathA, "mine")
}

func TestPullUnrelatedHistoryAsksInsteadOfTaking(t *testing.T) {
	v := newEnv(t)
	// This PC synced with a snapshot the folder doesn't have (e.g. a different sync folder), so
	// its base says nothing about NJ's snapshot: "local == base" must not mean "take remote".
	gone := syncdir.SnapshotName{Seq: 7, Time: v.clock, UUID: nyUUID}
	v.synced(gone, map[string]string{pathA: "a-mine"})
	v.writeLocal(pathA, "a-mine")
	v.remote(1, map[string]string{pathA: "a-nj"})
	v.prompt.choices = []Choice{ChooseLocal}
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if len(v.prompt.asked) != 1 || v.prompt.asked[0] != pathA {
		t.Fatalf("asked %v, want [%s]", v.prompt.asked, pathA)
	}
	v.want(pathA, "a-mine")
}

func TestPullRefusesPathsDifferingOnlyInCase(t *testing.T) {
	v := newEnv(t)
	lower := "Account/123#1/SavedVariables/details.lua"
	s1 := v.remote(1, map[string]string{lower: "d"})
	v.synced(s1, map[string]string{lower: "d"})
	v.writeLocal(lower, "d")
	// The other PC's copy has a different letter case. On Windows both names are the same file, so
	// writing Details.lua then deleting details.lua would lose it.
	v.remote(2, map[string]string{"Account/123#1/SavedVariables/Details.lua": "d2"})
	if _, err := v.eng.Pull(v.flavor, Options{}); err == nil || !strings.Contains(err.Error(), "letter case") {
		t.Fatalf("err = %v, want a letter-case error", err)
	}
	v.want(lower, "d")
	if st := v.state(); st.Base != s1.String() {
		t.Fatalf("base moved to %s", st.Base)
	}
}
