// Package engine runs push, pull, status, and restore for one flavor.
package engine

import (
	"maps"
	"slices"
)

// Action is what to do with one file when merging a snapshot.
type Action int

const (
	NoOp       Action = iota
	TakeRemote        // write (or delete) the snapshot's version locally
	KeepLocal         // leave the local version; the next push uploads it
	Conflict          // changed on both sides; the user decides
)

func (a Action) String() string {
	return [...]string{"no-op", "take", "keep", "conflict"}[a]
}

// Decide is the three-way rule for one file. Each argument is a hash; "" means the file is absent,
// so deletions follow the same rule as edits.
func Decide(base, local, remote string) Action {
	switch {
	case local == remote:
		return NoOp
	case local == base:
		return TakeRemote
	case remote == base:
		return KeepLocal
	default:
		return Conflict
	}
}

// Item is one file's decision.
type Item struct {
	Path   string
	Action Action
}

// Plan applies Decide to every path in base, local, or remote, sorted by path. No-ops are dropped.
func Plan(base, local, remote map[string]string) []Item {
	paths := map[string]bool{}
	for _, m := range []map[string]string{base, local, remote} {
		for p := range m {
			paths[p] = true
		}
	}
	var items []Item
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		if a := Decide(base[p], local[p], remote[p]); a != NoOp {
			items = append(items, Item{Path: p, Action: a})
		}
	}
	return items
}

// changed counts paths whose hash differs between a and b (including added or removed paths).
func changed(a, b map[string]string) int {
	n := 0
	for p, h := range a {
		if b[p] != h {
			n++
		}
	}
	for p := range b {
		if _, ok := a[p]; !ok {
			n++
		}
	}
	return n
}
