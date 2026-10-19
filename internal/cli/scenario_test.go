package cli

import "testing"

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
