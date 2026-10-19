package cli

import (
	"os"
	"path/filepath"
	"testing"
)

var start = map[string]string{svA: "a0", svB: "b0", svC: "c0"}

func TestScenarioLinkGetsEverything(t *testing.T) {
	_, ny := linkedPair(t, start)
	for p, c := range start {
		ny.want(p, c) // includes the "Area 52/Thrâll" character folder
	}
}

func TestScenarioDifferentAddonsOnEachPCBothSurvive(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.write(svB, "b-ny")
	contains(t, ny.mustRun("push"), "merged") // NY was behind: merges NJ's change, then pushes
	nj.mustRun("pull")
	for _, p := range []*pc{nj, ny} {
		p.want(svA, "a-nj")
		p.want(svB, "b-ny")
	}
}

func TestScenarioSameAddonOnBothAsks(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.write(svA, "a-ny")
	code, out := ny.run("l\n", "push")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, svA+" changed on NJ and on this PC")
	nj.mustRun("pull")
	nj.want(svA, "a-ny")
}

func TestScenarioAbortLeavesEverything(t *testing.T) {
	for _, stdin := range []string{"a\n", ""} { // explicit abort, and closed stdin
		nj, ny := linkedPair(t, start)
		nj.write(svA, "a-nj")
		nj.write(svB, "b-nj")
		nj.mustRun("push")
		ny.write(svA, "a-ny")
		code, out := ny.run(stdin, "pull")
		if code != 3 {
			t.Fatalf("stdin %q: exit %d\n%s", stdin, code, out)
		}
		ny.want(svA, "a-ny")
		ny.want(svB, "b0") // the non-conflicting change wasn't applied either
	}
}

func TestScenarioDeletionCarriesOver(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.remove(svC) // character deleted on NJ
	nj.mustRun("push")
	ny.mustRun("pull")
	if _, ok := ny.read(svC); ok {
		t.Fatal("deleted character's settings still on NY")
	}
}

func TestScenarioRestoreUndoesPull(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.mustRun("pull")
	ny.want(svA, "a-nj")
	contains(t, ny.mustRun("restore"), "restored backup")
	ny.want(svA, "a0")
}

func TestScenarioGameRunningRefusesPull(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.procs.running = true
	if code, _ := ny.run("", "pull"); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	ny.want(svA, "a0")
}

// fork has NJ and NY both push seq 2 on top of seq 1 without seeing each other's push (cloud lag):
// NJ's snapshot is hidden while NY pushes, then reappears. NY's is newer, so NJ is the one behind.
func fork(t *testing.T, njEdit, nyEdit [2]string) (nj, ny *pc) {
	t.Helper()
	nj, ny = linkedPair(t, start)
	nj.write(njEdit[0], njEdit[1])
	nj.mustRun("push")
	snaps := filepath.Join(nj.sync, "wtfsync", "retail", "snapshots")
	hidden := filepath.Join(t.TempDir(), "hidden")
	all, _ := filepath.Glob(filepath.Join(snaps, "000002_*"))
	if len(all) != 1 {
		t.Fatalf("NJ snapshots: %v", all)
	}
	if err := os.Rename(all[0], hidden); err != nil {
		t.Fatal(err)
	}
	ny.write(nyEdit[0], nyEdit[1])
	contains(t, ny.mustRun("push"), "000002_")
	if err := os.Rename(hidden, all[0]); err != nil {
		t.Fatal(err)
	}
	return nj, ny
}

func TestScenarioForkKeepsBothPCsChanges(t *testing.T) {
	nj, ny := fork(t, [2]string{svA, "a-nj"}, [2]string{svB, "b-ny"})
	nj.mustRun("pull")
	nj.want(svA, "a-nj") // NJ's own edit must survive merging NY's sibling snapshot
	nj.want(svB, "b-ny")
	nj.mustRun("push")
	ny.mustRun("pull")
	ny.want(svA, "a-nj")
	ny.want(svB, "b-ny")
}

func TestScenarioForkSameFileAsks(t *testing.T) {
	nj, _ := fork(t, [2]string{svA, "a-nj"}, [2]string{svA, "a-ny"})
	if code, out := nj.run("", "pull"); code != 3 {
		t.Fatalf("closed stdin: exit %d, want 3 (conflict)\n%s", code, out)
	}
	nj.want(svA, "a-nj")
	code, out := nj.run("l\n", "pull")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, svA+" changed on NY and on this PC")
	nj.want(svA, "a-nj")
}

func TestScenarioRestoreDryRunChangesNothing(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.mustRun("pull")
	contains(t, ny.mustRun("restore", "--dry-run"), svA)
	ny.want(svA, "a-nj")
}

func TestScenarioRestoreCanBeUndone(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	ny.mustRun("pull")
	ny.write(svA, "a-session") // played after pulling; not pushed yet
	contains(t, ny.mustRun("restore"), "undo with: wtfsync restore")
	ny.want(svA, "a0")
	ny.mustRun("restore") // the newest set is now the one restore made
	ny.want(svA, "a-session")
}
