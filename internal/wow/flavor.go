// Package wow knows where World of Warcraft keeps its files and whether the game is running.
package wow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Flavor is one installed game flavor, e.g. retail in <root>/_retail_.
type Flavor struct {
	Name string // "retail", "classic", "classic_era", "ptr", ...
	Dir  string // <root>/_retail_
}

func (f Flavor) WTFDir() string    { return filepath.Join(f.Dir, "WTF") }
func (f Flavor) AddOnsDir() string { return filepath.Join(f.Dir, "Interface", "AddOns") }

// Flavors lists the flavors under an install root: every _<name>_ folder that contains a WTF
// folder, in folder-name order.
func Flavors(root string) ([]Flavor, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read WoW install folder: %w", err)
	}
	var out []Flavor
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || len(n) < 3 || !strings.HasPrefix(n, "_") || !strings.HasSuffix(n, "_") {
			continue
		}
		dir := filepath.Join(root, n)
		if fi, err := os.Stat(filepath.Join(dir, "WTF")); err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, Flavor{Name: n[1 : len(n)-1], Dir: dir})
	}
	return out, nil
}

// DefaultInstallDirs are the usual Windows install roots, most likely first.
func DefaultInstallDirs() []string {
	return []string{
		`C:\Program Files (x86)\World of Warcraft`,
		`C:\Program Files\World of Warcraft`,
	}
}
