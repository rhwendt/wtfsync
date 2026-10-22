package app

import (
	"fmt"

	"github.com/rhwendt/wtfsync/internal/setup"
)

// ResultLines describes a setup run for the wizard's last page.
func ResultLines(r setup.Result) []string {
	lines := []string{}
	if r.Rerun && !r.Moved {
		lines = append(lines, "Settings saved; sync history kept.")
	}
	for _, f := range r.Synced {
		rep := f.Report
		switch {
		case f.Err != nil:
			lines = append(lines, fmt.Sprintf("%s: %s", f.Flavor, friendly(f.Flavor, f.Err)))
		case rep.Snapshot != "":
			lines = append(lines, fmt.Sprintf("%s: uploaded %d file(s)", f.Flavor, rep.Pushed))
		case rep.Merged != "":
			lines = append(lines, fmt.Sprintf("%s: got %s's settings (%d file(s))", f.Flavor, rep.From, len(rep.Items)))
		default:
			lines = append(lines, f.Flavor+": nothing to do")
		}
	}
	return lines
}
