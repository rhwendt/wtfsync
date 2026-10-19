// Package app is the tray app's logic, kept free of any UI toolkit so it can be tested anywhere:
// what state each flavor is in, which icon and menu to show, what the menu items do, the 60 s
// tick, update offers, and which notifications to raise. Package ui only renders it.
package app

import (
	"errors"
	"fmt"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// IconState is the tray icon, most severe state across flavors.
type IconState int

const (
	Idle        IconState = iota // every flavor in sync
	NewSettings                  // another PC pushed newer settings
	Attention                    // error, refused action, pending push, missing addons, not set up
	Syncing                      // an action is running
)

// FlavorView is one flavor as the tray shows it.
type FlavorView struct {
	Name   string
	Status engine.Status
	Err    string // the last action's error for this flavor; "" when it went fine
}

// newer reports whether another PC has pushed something this PC hasn't synced.
func (f FlavorView) newer() bool {
	return f.Status.Reachable && f.Status.Newest != "" && f.Status.Newest != f.Status.Base
}

func (f FlavorView) missing() int {
	n := 0
	for _, o := range f.Status.MissingAddons {
		n += len(o.Addons)
	}
	return n
}

// issue is what's wrong with the flavor in a few words, most important first; "" if nothing.
func (f FlavorView) issue() (string, IconState) {
	s := f.Status
	switch {
	case f.Err != "":
		return f.Err, Attention
	case s.PendingPush:
		return "push pending (sync folder unreachable)", Attention
	case !s.Reachable:
		return "sync folder unreachable", Attention
	case f.missing() > 0:
		return fmt.Sprintf("%d addon(s) missing here", f.missing()), Attention
	case f.newer():
		return s.NewestBy + " has newer settings", NewSettings
	}
	return "", Idle
}

// View is everything the tray renders.
type View struct {
	SetUp   bool
	Busy    string // "pulling", "pushing", "restoring", "setting up", "updating"; "" when idle
	Icon    IconState
	Tooltip string
	Flavors []FlavorView
	Missing int    // missing addons across flavors
	Update  string // newer version on offer, e.g. "v0.6.0"; "" when none
}

const tooltipMax = 127 // Windows truncates tray tooltips beyond this

func summarize(v *View) {
	switch {
	case !v.SetUp:
		v.Icon, v.Tooltip = Attention, "wtfsync — not set up"
		return
	case v.Busy != "":
		v.Icon, v.Tooltip = Syncing, "wtfsync — "+v.Busy+"…"
		return
	}
	v.Icon = Idle
	var issues []string
	for _, f := range v.Flavors {
		v.Missing += f.missing()
		text, state := f.issue()
		if text == "" {
			continue
		}
		issues = append(issues, f.Name+": "+text)
		v.Icon = max(v.Icon, state)
	}
	switch len(issues) {
	case 0:
		v.Tooltip = "wtfsync — in sync"
	case 1:
		v.Tooltip = issues[0]
	default:
		v.Tooltip = fmt.Sprintf("%s (+%d more)", issues[0], len(issues)-1)
	}
	if r := []rune(v.Tooltip); len(r) > tooltipMax {
		v.Tooltip = string(r[:tooltipMax-1]) + "…"
	}
}

// MenuItem is one tray menu entry. ID "-" is a separator.
type MenuItem struct {
	ID      string
	Label   string
	Enabled bool
}

// Menu builds the tray menu for v. ui maps IDs to actions.
func Menu(v View) []MenuItem {
	if !v.SetUp {
		return []MenuItem{{"setup", "Set up wtfsync…", v.Busy == ""}, {"-", "", false}, {"quit", "Quit", true}}
	}
	idle := v.Busy == ""
	m := []MenuItem{
		{"push", "Push", idle},
		{"pull", "Pull", idle},
		{"-", "", false},
		{"status", "Status…", true},
		{"restore", "Restore last backup…", idle},
	}
	if v.Missing > 0 {
		m = append(m, MenuItem{"addons", fmt.Sprintf("Missing addons (%d)…", v.Missing), true})
	}
	m = append(m, MenuItem{"settings", "Settings…", idle})
	if v.Update != "" {
		m = append(m, MenuItem{"update", "Update to " + v.Update, idle})
	}
	return append(m, MenuItem{"-", "", false}, MenuItem{"quit", "Quit", true})
}

// friendly turns an engine error into the words a notification shows.
func friendly(flavor string, err error) string {
	switch {
	case errors.Is(err, engine.ErrGameRunning):
		return fmt.Sprintf("Close WoW (%s) first", flavor)
	case errors.Is(err, engine.ErrAborted):
		return "Conflict not resolved; nothing changed"
	}
	return err.Error()
}
