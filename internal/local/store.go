// Package local stores this PC's wtfsync data: settings, last-synced state, and backups.
package local

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rhwendt/wtfsync/internal/fsutil"
)

var ErrNoSettings = errors.New("wtfsync isn't set up on this PC (run `wtfsync init` or `wtfsync link`)")

type Settings struct {
	MachineUUID string          `json:"machineUUID"`
	MachineName string          `json:"machineName"` // display label only; never identity
	SyncFolder  string          `json:"syncFolder"`
	WoWDir      string          `json:"wowDir"`
	Flavors     map[string]bool `json:"flavors"` // flavor → on; what syncs when AllFlavors is off

	// AllFlavors syncs every flavor, including ones installed later. nil in settings written
	// before it existed: see All.
	AllFlavors *bool  `json:"allFlavors,omitempty"`
	Appearance string `json:"appearance,omitempty"` // "system" (or ""), "light", "dark"
}

// All reports whether every flavor syncs. Settings written before AllFlavors existed count as all
// when no listed flavor was switched off, so nobody's choices change on upgrade.
func (s Settings) All() bool {
	if s.AllFlavors != nil {
		return *s.AllFlavors
	}
	for _, on := range s.Flavors {
		if !on {
			return false
		}
	}
	return true
}

// Enabled reports whether flavor syncs. When not all flavors sync, one that isn't listed (installed
// since) is off until switched on.
func (s Settings) Enabled(flavor string) bool {
	return s.All() || s.Flavors[flavor]
}

// Theme is the appearance to use: "system", "light" or "dark". Anything else reads as system.
func (s Settings) Theme() string {
	switch s.Appearance {
	case "light", "dark":
		return s.Appearance
	}
	return "system"
}

// FlavorState is what this PC last synced for one flavor: the base of the three-way compare.
type FlavorState struct {
	Base        string            `json:"base"`  // snapshot name; "" = never synced
	Files       map[string]string `json:"files"` // that snapshot's file hashes
	PendingPush bool              `json:"pendingPush"`

	// Set by restoring a snapshot, cleared when a sync records a new base: the snapshot put back
	// and its file hashes, so the window can mark it current until the files change again.
	Restored      string            `json:"restored,omitempty"`
	RestoredFiles map[string]string `json:"restoredFiles,omitempty"`
}

// Store is the local data folder (%APPDATA%\wtfsync).
type Store struct {
	Dir string
}

func (s Store) LoadSettings() (Settings, error) {
	var v Settings
	err := readJSON(filepath.Join(s.Dir, "settings.json"), &v)
	if errors.Is(err, fs.ErrNotExist) {
		return v, ErrNoSettings
	}
	return v, err
}

func (s Store) SaveSettings(v Settings) error {
	return fsutil.WriteJSON(filepath.Join(s.Dir, "settings.json"), v)
}

func (s Store) statePath(flavor string) string {
	return filepath.Join(s.Dir, "state", flavor+".json")
}

func (s Store) LoadState(flavor string) (FlavorState, error) {
	var v FlavorState
	err := readJSON(s.statePath(flavor), &v)
	if errors.Is(err, fs.ErrNotExist) {
		return FlavorState{}, nil
	}
	return v, err
}

func (s Store) SaveState(flavor string, v FlavorState) error {
	return fsutil.WriteJSON(s.statePath(flavor), v)
}

func readJSON(p string, v any) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	return nil
}

// NewUUID returns a random version 4 UUID.
func NewUUID() string {
	var b [16]byte
	rand.Read(b[:]) // never fails (Go 1.24+)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
