package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// printReport prints a pull or push result. op is "pull" or "push".
func printReport(w io.Writer, op string, r engine.Report, dry bool) {
	if r.Merged != "" {
		verb := "pulled"
		switch {
		case op == "push" && dry:
			verb = "would merge"
		case op == "push":
			verb = "merged"
		case dry:
			verb = "would pull"
		}
		fmt.Fprintf(w, "%s: %s %s from %s\n", r.Flavor, verb, r.Merged, r.From)
		for _, it := range r.Items {
			fmt.Fprintf(w, "  %-8s %s\n", it.Action, it.Path)
		}
		if r.Backup != "" {
			fmt.Fprintf(w, "  backup %s (undo with: wtfsync restore --flavor %s)\n", r.Backup, r.Flavor)
		}
		if len(r.MissingAddons) > 0 {
			fmt.Fprintf(w, "  %s has addons not installed here: %s\n", r.From, strings.Join(r.MissingAddons, ", "))
		}
	}
	switch {
	case op == "pull":
		if r.UpToDate {
			fmt.Fprintf(w, "%s: up to date\n", r.Flavor)
		}
	case r.Pending:
		fmt.Fprintf(w, "%s: sync folder unreachable (%s); push will be retried\n", r.Flavor, r.PendingReason)
	case r.UpToDate:
		fmt.Fprintf(w, "%s: nothing to push\n", r.Flavor)
	case dry:
		fmt.Fprintf(w, "%s: would push %d changed file(s)\n", r.Flavor, r.Pushed)
	default:
		fmt.Fprintf(w, "%s: pushed %s (%d file(s) changed)\n", r.Flavor, r.Snapshot, r.Pushed)
	}
}

func printStatus(w io.Writer, s engine.Status) {
	base := s.Base
	if base == "" {
		base = "never synced"
	}
	fmt.Fprintf(w, "%s: base %s; %d local change(s)", s.Flavor, base, s.LocalChanges)
	switch {
	case !s.Reachable:
		fmt.Fprint(w, "; sync folder unreachable")
	case s.Newest == "":
		fmt.Fprint(w, "; no snapshots yet")
	case s.Newest == s.Base:
		fmt.Fprint(w, "; up to date with the sync folder")
	default:
		fmt.Fprintf(w, "; newer snapshot %s from %s", s.Newest, s.NewestBy)
	}
	if s.PendingPush {
		fmt.Fprint(w, "; push pending")
	}
	fmt.Fprintln(w)
}
