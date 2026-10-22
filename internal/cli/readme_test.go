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
	if len(cmds) != 8 || cmds[0] != "init" || len(flags) != 7 || flags[0] != "--flavor" {
		t.Fatalf("commands %v, flags %v", cmds, flags)
	}
}

func TestReadmeDocumentsEveryCommandAndFlag(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := commandReference(string(b))
	if !ok {
		t.Fatal(`README has no "## Command reference" section`)
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

func TestCommandReferenceFoundWithWindowsLineEndings(t *testing.T) {
	// Git on Windows checks the README out with CRLF line endings.
	readme := "# x\r\n\r\n## Command reference\r\n\r\n### `wtfsync push`\r\n\r\n## Build\r\n"
	ref, ok := commandReference(readme)
	if !ok || !strings.Contains(ref, "### `wtfsync push`") || strings.Contains(ref, "Build") {
		t.Fatalf("got %q, %v", ref, ok)
	}
}

// commandReference is the README's "## Command reference" section, up to the next "## ".
func commandReference(readme string) (string, bool) {
	readme = strings.ReplaceAll(readme, "\r\n", "\n")
	_, ref, ok := strings.Cut(readme, "\n## Command reference\n")
	if !ok {
		return "", false
	}
	if end := strings.Index(ref, "\n## "); end >= 0 {
		ref = ref[:end]
	}
	return ref, true
}
