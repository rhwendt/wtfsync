package cli

import (
	"testing"

	"github.com/rhwendt/wtfsync/internal/lock"
)

func TestSyncRefusedWhileAnotherSyncRuns(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	held, err := lock.Acquire(ny.dataDir) // e.g. the tray is pulling
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"pull", "push", "restore"} {
		code, out := ny.run("", op)
		if code != 2 {
			t.Fatalf("%s: exit %d, want 2\n%s", op, code, out)
		}
		contains(t, out, "another wtfsync sync is running")
	}
	ny.want(svA, "a0")
	contains(t, ny.mustRun("status"), "retail:") // status still answers
	held.Release()
	ny.mustRun("pull")
	ny.want(svA, "a-nj")
}

func TestVersion(t *testing.T) {
	nj, _ := newPair(t)
	contains(t, nj.mustRun("version"), "wtfsync dev")
}
