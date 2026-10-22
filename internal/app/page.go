package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// MainPage is everything the main window shows.
type MainPage struct {
	SetUp      bool
	Busy       string // what's running ("syncing classic"); "" when idle
	PC         string // this PC's name
	Version    string
	Update     string // newer version on offer; "" when none
	Appearance string // "system", "light" or "dark"
	Flavors    []FlavorPage
}

// FlavorPage is one flavor: its row in the list and its page.
type FlavorPage struct {
	Name        string
	Badge       Badge
	Line        string // "Last synced today 03:51 · newest from leviathan"
	Points      []PointItem
	Unreachable bool // the cloud folder can't be reached, so there are no points to list
	Missing     []MissingGroup
}

// Badge is a flavor's state in a few characters (the list) and in words (its page).
type Badge struct {
	Kind  string // "error", "newer", "changes", "new" or "ok": the colour
	Short string // "!", "↓", "↑ 26", "new", "✓"
	Words string // "Cloud folder unreachable", "leviathan has newer settings", "26 changes to sync", …
}

// PointItem is one restore point in the list.
type PointItem struct {
	Name    string // snapshot name, passed back to RestorePoint
	When    string // "today 03:51"
	From    string // the PC that synced it
	Current bool   // this PC's files match it: nothing to restore
}

// MissingGroup is the addons one other PC has that this one doesn't.
type MissingGroup struct {
	PC     string
	Addons []MissingAddon
}

type MissingAddon struct {
	Name    string
	URL     string // download page; "" when unknown
	Folders string // "ElvUI, ElvUI_Options"
}

// Main is the main window's data. It reads what the last refresh found; it never refreshes or calls Changed, because the window reloads it on every Changed.
func (a *App) Main() MainPage {
	v := a.View()
	m := MainPage{SetUp: v.SetUp, Busy: v.Busy, Version: a.d.Version, Update: v.Update, Appearance: "system", Flavors: []FlavorPage{}}
	if s, ok, _ := a.Settings(); ok {
		m.PC, m.Appearance = s.MachineName, s.Theme()
	}
	now := a.d.Now()
	for _, f := range v.Flavors {
		m.Flavors = append(m.Flavors, FlavorPage{
			Name:        f.Name,
			Badge:       badge(f),
			Line:        statusLine(f.Status, now),
			Points:      points(f.Status, now),
			Unreachable: !f.Status.Reachable,
			Missing:     missingGroups(f.Status),
		})
	}
	return m
}

func points(s engine.Status, now time.Time) []PointItem {
	out := []PointItem{}
	for _, p := range s.Points {
		out = append(out, PointItem{Name: p.Name, When: when(p.Time, now), From: p.From, Current: p.Current})
	}
	return out
}

func missingGroups(s engine.Status) []MissingGroup {
	out := []MissingGroup{}
	for _, o := range s.MissingAddons {
		g := MissingGroup{PC: o.Machine, Addons: []MissingAddon{}}
		for _, m := range o.Addons {
			g.Addons = append(g.Addons, MissingAddon{Name: m.Name, URL: m.URL, Folders: strings.Join(m.Folders, ", ")})
		}
		out = append(out, g)
	}
	return out
}

// badge is the flavor's most important state.
func badge(f FlavorView) Badge {
	s := f.Status
	switch {
	case f.Err != "":
		return Badge{"error", "!", f.Err}
	case s.PendingPush:
		return Badge{"error", "!", "Sync pending: cloud folder unreachable"}
	case !s.Reachable:
		return Badge{"error", "!", "Cloud folder unreachable"}
	case f.newer():
		return Badge{"newer", "↓", s.NewestBy + " has newer settings"}
	case s.LocalChanges > 0:
		return Badge{"changes", fmt.Sprintf("↑ %d", s.LocalChanges), plural(s.LocalChanges, "change") + " to sync"}
	case s.Base == "":
		return Badge{"new", "new", "Not synced yet"}
	}
	return Badge{"ok", "✓", "In sync"}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// statusLine says when the flavor last synced and what's newest, in local time.
func statusLine(s engine.Status, now time.Time) string {
	base, hasBase := syncdir.ParseName(s.Base)
	newest, hasNewest := syncdir.ParseName(s.Newest)
	switch {
	case !hasBase && hasNewest:
		return fmt.Sprintf("Never synced · %s pushed %s", s.NewestBy, when(newest.Time, now))
	case !hasBase:
		return "Never synced"
	case !hasNewest:
		return "Last synced " + when(base.Time, now)
	case s.Base == s.Newest && s.BaseBy != "":
		return fmt.Sprintf("Last synced %s · newest from %s", when(base.Time, now), s.BaseBy)
	case s.Base == s.Newest:
		return "Last synced " + when(base.Time, now)
	}
	return fmt.Sprintf("Last synced %s · %s pushed %s", when(base.Time, now), s.NewestBy, when(newest.Time, now))
}

// when is t in now's time zone, as a person would say it.
func when(t, now time.Time) string {
	t = t.In(now.Location())
	sameDay := func(a, b time.Time) bool {
		ay, am, ad := a.Date()
		by, bm, bd := b.Date()
		return ay == by && am == bm && ad == bd
	}
	switch {
	case sameDay(t, now):
		return "today " + t.Format("15:04")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return "yesterday " + t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2, 15:04")
	}
	return t.Format("Jan 2 2006, 15:04")
}
