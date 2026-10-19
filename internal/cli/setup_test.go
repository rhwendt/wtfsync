package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhwendt/wtfsync/internal/local"
)

func TestInitCreatesSyncDataAndPushes(t *testing.T) {
	nj, _ := newPair(t)
	nj.write(svA, "a")
	out := nj.initCmd()
	contains(t, out, "retail: pushed 000001_")
	if _, err := os.Stat(filepath.Join(nj.sync, "wtfsync", "wtfsync.json")); err != nil {
		t.Fatal(err)
	}
	s, err := local.Store{Dir: nj.dataDir}.LoadSettings()
	if err != nil || s.MachineName != "NJ" || s.SyncFolder != nj.sync || !s.Flavors["retail"] || s.MachineUUID == "" {
		t.Fatalf("settings %+v, %v", s, err)
	}
}

func TestInitRefusesFolderWithData(t *testing.T) {
	nj, ny := newPair(t)
	nj.initCmd()
	code, out := ny.run("", "init", "--wow-dir", ny.wowDir, "--sync-dir", ny.sync, "--name", "NY")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, out, "wtfsync link")
}

func TestInitTwiceOnSamePC(t *testing.T) {
	nj, _ := newPair(t)
	nj.initCmd()
	code, out := nj.run("", "init", "--wow-dir", nj.wowDir, "--sync-dir", nj.sync, "--name", "NJ")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, out, "already set up")
}

func TestLinkNeedsInitializedFolder(t *testing.T) {
	_, ny := newPair(t)
	code, out := ny.link("")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, out, "wtfsync init")
}

func TestLinkAsksOnceAndTakesOtherPC(t *testing.T) {
	nj, ny := newPair(t)
	nj.write(svA, "nj")
	nj.initCmd()
	ny.write(svA, "ny")
	code, out := ny.link("1\n")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, "both have settings")
	ny.want(svA, "nj")
}

func TestLinkUUIDsDifferEvenWithSameName(t *testing.T) {
	nj, ny := newPair(t)
	nj.initCmd()
	ny.run("", "link", "--wow-dir", ny.wowDir, "--sync-dir", ny.sync, "--name", "NJ")
	a, _ := local.Store{Dir: nj.dataDir}.LoadSettings()
	b, _ := local.Store{Dir: ny.dataDir}.LoadSettings()
	if a.MachineUUID == b.MachineUUID || b.MachineName != "NJ" {
		t.Fatalf("uuids %s / %s", a.MachineUUID, b.MachineUUID)
	}
}

func TestInitDryRunWritesNothing(t *testing.T) {
	nj, _ := newPair(t)
	nj.write(svA, "a")
	code, out := nj.run("", "init", "--dry-run", "--wow-dir", nj.wowDir, "--sync-dir", nj.sync)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, svA)
	contains(t, out, "nothing written")
	if _, err := os.Stat(filepath.Join(nj.sync, "wtfsync")); !os.IsNotExist(err) {
		t.Fatal("dry run created sync data")
	}
	if _, err := os.Stat(nj.dataDir); !os.IsNotExist(err) {
		t.Fatal("dry run created local data")
	}
}

func TestNameDefaultsToHostname(t *testing.T) {
	nj, _ := newPair(t)
	code, out := nj.run("\n", "init", "--wow-dir", nj.wowDir, "--sync-dir", nj.sync)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	s, _ := local.Store{Dir: nj.dataDir}.LoadSettings()
	if s.MachineName != "DESKTOP-NJ" {
		t.Fatalf("name = %q", s.MachineName)
	}
}

func TestWoWDirDefaultsToDetectedInstall(t *testing.T) {
	nj, _ := newPair(t)
	code, out := nj.run("\n", "init", "--sync-dir", nj.sync, "--name", "NJ") // Enter accepts the detected folder
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	s, _ := local.Store{Dir: nj.dataDir}.LoadSettings()
	if s.WoWDir != nj.wowDir {
		t.Fatalf("wowDir = %q", s.WoWDir)
	}
}

func TestUnknownCommandAndNoArgs(t *testing.T) {
	nj, _ := newPair(t)
	if code, _ := nj.run("", "frobnicate"); code != 1 {
		t.Fatalf("unknown command exit %d", code)
	}
	if code, _ := nj.run(""); code != 1 {
		t.Fatalf("no args exit %d", code)
	}
	if code, out := nj.run("", "help"); code != 0 {
		t.Fatalf("help exit %d", code)
	} else {
		contains(t, out, "usage: wtfsync")
	}
}

func TestTypedFolderMayBeQuoted(t *testing.T) {
	// Explorer's "Copy as path" wraps the path in double quotes; people paste it as is.
	for _, quote := range []string{`"`, `'`} {
		nj, _ := newPair(t)
		stdin := quote + nj.wowDir + quote + "\n"
		code, out := nj.run(stdin, "init", "--sync-dir", nj.sync, "--name", "NJ")
		if code != 0 {
			t.Fatalf("quote %s: exit %d\n%s", quote, code, out)
		}
		s, _ := local.Store{Dir: nj.dataDir}.LoadSettings()
		if s.WoWDir != nj.wowDir {
			t.Fatalf("quote %s: wowDir = %q", quote, s.WoWDir)
		}
	}
}

func TestSetupListsEveryWoWInstallFound(t *testing.T) {
	nj, _ := newPair(t)
	second := filepath.Join(t.TempDir(), "World of Warcraft")
	if err := os.MkdirAll(filepath.Join(second, "_classic_era_", "WTF"), 0o755); err != nil {
		t.Fatal(err)
	}
	notWoW := t.TempDir() // exists, but has no flavor folders
	nj.installDirs = []string{notWoW, nj.wowDir, filepath.Join(t.TempDir(), "missing"), second}
	code, out := nj.run("2\n", "init", "--sync-dir", nj.sync, "--name", "NJ")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	contains(t, out, "1) "+nj.wowDir)
	contains(t, out, "2) "+second)
	if strings.Contains(out, notWoW) {
		t.Fatalf("listed a folder with no WoW in it:\n%s", out)
	}
	s, _ := local.Store{Dir: nj.dataDir}.LoadSettings()
	if s.WoWDir != second {
		t.Fatalf("wowDir = %q, want %q", s.WoWDir, second)
	}
}
