package app

import (
	"fmt"
	"strings"

	"github.com/rhwendt/wtfsync/internal/setup"
)

// StatusRow is one line of the Status window.
type StatusRow struct {
	Flavor string
	State  string // "in sync", or what's wrong
	Base   string // last synced snapshot, "never synced"
	Newest string // "<snapshot> from <PC>", or "none"
	Local  string // "2 local change(s)"
}

func StatusRows(v View) []StatusRow {
	var rows []StatusRow
	for _, f := range v.Flavors {
		s := f.Status
		r := StatusRow{Flavor: f.Name, State: "in sync", Base: s.Base, Newest: "none", Local: fmt.Sprintf("%d local change(s)", s.LocalChanges)}
		if text, _ := f.issue(); text != "" {
			r.State = text
		}
		if r.Base == "" {
			r.Base = "never synced"
		}
		if s.Newest != "" {
			r.Newest = s.Newest + " from " + s.NewestBy
		}
		rows = append(rows, r)
	}
	return rows
}

// AddonRow is one missing addon in the Missing addons window.
type AddonRow struct {
	Flavor  string
	Machine string // the PC that has it
	Name    string
	Folders string // "ElvUI, ElvUI_Options"
	URL     string // "" when unknown
}

func AddonRows(v View) []AddonRow {
	var rows []AddonRow
	for _, f := range v.Flavors {
		for _, o := range f.Status.MissingAddons {
			for _, m := range o.Addons {
				rows = append(rows, AddonRow{Flavor: f.Name, Machine: o.Machine, Name: m.Name, Folders: strings.Join(m.Folders, ", "), URL: m.URL})
			}
		}
	}
	return rows
}

// ResultLines describes a setup run for the wizard's last page.
func ResultLines(r setup.Result) []string {
	var lines []string
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
