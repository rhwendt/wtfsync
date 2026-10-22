package cli

import (
	"os"
	"strings"
	"testing"
)

// usageNames lists the commands and flags in the CLI's usage text.
func usageNames(u string) (cmds, flags []string) {
	inCmds := false
	for _, line := range strings.Split(u, "\n") {
		switch {
		case line == "commands:":
			inCmds = true
		case strings.TrimSpace(line) == "":
			inCmds = false
		case strings.HasPrefix(line, "  --"):
			flags = append(flags, strings.Fields(line)[0])
		case inCmds && strings.HasPrefix(line, "  "):
			cmds = append(cmds, strings.Fields(line)[0])
		}
	}
	return cmds, flags
}

func TestUsageNamesFindsEverything(t *testing.T) {
	cmds, flags := usageNames(usage)
	if len(cmds) != 7 || cmds[0] != "init" || len(flags) != 7 || flags[0] != "--flavor" {
		t.Fatalf("commands %v, flags %v", cmds, flags)
	}
}

func TestReadmeDocumentsEveryCommandAndFlag(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, ref, ok := strings.Cut(string(b), "\n## Command reference\n")
	if !ok {
		t.Fatal(`README has no "## Command reference" section`)
	}
	if end := strings.Index(ref, "\n## "); end >= 0 {
		ref = ref[:end]
	}
	cmds, flags := usageNames(usage)
	for _, c := range cmds {
		if !strings.Contains(ref, "### `wtfsync "+c+"`") {
			t.Errorf("Command reference has no ### `wtfsync %s` heading", c)
		}
	}
	for _, f := range flags {
		if !strings.Contains(ref, "`"+f) {
			t.Errorf("Command reference doesn't mention %s", f)
		}
	}
}
