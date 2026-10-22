package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhwendt/wtfsync/internal/local"
)

func TestPushPullRoundTrip(t *testing.T) {
	nj, ny := linkedPair(t, map[string]string{svA: "a0"})
	nj.write(svA, "a1")
	contains(t, nj.mustRun("push"), "retail: pushed 000002_")
	contains(t, ny.mustRun("pull"), "retail: pulled 000002_")
	ny.want(svA, "a1")
	contains(t, ny.mustRun("pull"), "retail: up to date")
}

func TestSyncGetsTheOtherPCsChangesAndSendsItsOwn(t *testing.T) {
	nj, ny := linkedPair(t, map[string]string{svA: "a0"})
	nj.write(svA, "a1")
	nj.mustRun("sync")
	ny.write(svB, "b-ny")
	contains(t, ny.mustRun("sync"), "retail: pushed 000003_")
	ny.want(svA, "a1")
	nj.mustRun("sync")
	nj.want(svB, "b-ny")
}

func TestPushRefusedWhileGameRunning(t *testing.T) {
	nj, _ := linkedPair(t, map[string]string{svA: "a0"})
	nj.procs.running = true
	code, out := nj.run("", "push")
	if code != 2 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, "close WoW")
}

func TestUnknownFlavor(t *testing.T) {
	nj, _ := linkedPair(t, nil)
	code, out := nj.run("", "push", "--flavor", "cataclysm")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, out, `unknown flavor "cataclysm"`)
}

func TestNewFlavorIncludedDisabledFlavorSkipped(t *testing.T) {
	nj, _ := linkedPair(t, map[string]string{svA: "a0"})
	classic := filepath.Join(nj.wowDir, "_classic_", "WTF", filepath.FromSlash(svB))
	os.MkdirAll(filepath.Dir(classic), 0o755)
	os.WriteFile(classic, []byte("b"), 0o644)
	contains(t, nj.mustRun("push"), "classic: pushed")

	store := local.Store{Dir: nj.dataDir}
	s, _ := store.LoadSettings()
	off := false
	s.AllFlavors = &off
	s.Flavors["classic"] = false
	store.SaveSettings(s)
	os.WriteFile(classic, []byte("b2"), 0o644)
	if out := nj.mustRun("push"); strings.Contains(out, "classic:") {
		t.Fatalf("disabled flavor pushed:\n%s", out)
	}
}

func TestStatusOutput(t *testing.T) {
	nj, ny := linkedPair(t, map[string]string{svA: "a0"})
	contains(t, ny.mustRun("status"), "up to date with the sync folder")
	nj.write(svA, "a1")
	nj.mustRun("push")
	ny.write(svB, "new")
	out := ny.mustRun("status")
	contains(t, out, "1 local change(s)")
	contains(t, out, "from NJ")
}

func TestPullDryRunChangesNothing(t *testing.T) {
	nj, ny := linkedPair(t, map[string]string{svA: "a0"})
	nj.write(svA, "a1")
	nj.mustRun("push")
	out := ny.mustRun("pull", "--dry-run")
	contains(t, out, "would pull")
	contains(t, out, "take     "+svA)
	ny.want(svA, "a0")
}

func TestPendingPushRetriedOnNextCommand(t *testing.T) {
	nj, ny := linkedPair(t, map[string]string{svA: "a0"})
	ny.write(svA, "a-ny")
	offline := ny.sync + "-offline"
	if err := os.Rename(ny.sync, offline); err != nil {
		t.Fatal(err)
	}
	contains(t, ny.mustRun("push"), "push will be retried")
	contains(t, ny.mustRun("status"), "push pending")
	if err := os.Rename(offline, ny.sync); err != nil {
		t.Fatal(err)
	}
	out := ny.mustRun("status")
	contains(t, out, "retrying pending push")
	contains(t, out, "retail: pushed")
	nj.mustRun("pull")
	nj.want(svA, "a-ny")
}
