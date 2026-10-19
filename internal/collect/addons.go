package collect

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Addon is one installed addon as described by its .toc file.
type Addon struct {
	Folder  string `json:"folder"`
	Version string `json:"version,omitempty"`
	Curse   string `json:"curse,omitempty"`
	Wago    string `json:"wago,omitempty"`
	WoWI    string `json:"wowi,omitempty"`
}

// Addons lists the addon folders under Interface/AddOns that have a .toc file, sorted by folder.
// A missing AddOns folder yields nil.
func Addons(addonsDir string) ([]Addon, error) {
	entries, err := os.ReadDir(addonsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read addons: %w", err)
	}
	var out []Addon
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		toc, err := findTOC(filepath.Join(addonsDir, e.Name()), e.Name())
		if err != nil {
			return nil, err
		}
		if toc == "" {
			continue
		}
		a, err := parseTOC(toc)
		if err != nil {
			return nil, err
		}
		a.Folder = e.Name()
		out = append(out, a)
	}
	return out, nil
}

// findTOC prefers <folder>.toc, else the first <folder>_*.toc (e.g. ElvUI_Mainline.toc).
func findTOC(dir, folder string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var alt []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.EqualFold(filepath.Ext(n), ".toc") {
			continue
		}
		stem := strings.TrimSuffix(n, filepath.Ext(n))
		if strings.EqualFold(stem, folder) {
			return filepath.Join(dir, n), nil
		}
		if len(stem) > len(folder) && strings.EqualFold(stem[:len(folder)+1], folder+"_") {
			alt = append(alt, n)
		}
	}
	if len(alt) == 0 {
		return "", nil
	}
	sort.Strings(alt)
	return filepath.Join(dir, alt[0]), nil
}

func parseTOC(p string) (Addon, error) {
	f, err := os.Open(p)
	if err != nil {
		return Addon{}, err
	}
	defer f.Close()
	var a Addon
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if !strings.HasPrefix(line, "##") {
			continue
		}
		key, val, ok := strings.Cut(line[2:], ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "version":
			a.Version = val
		case "x-curse-project-id":
			a.Curse = val
		case "x-wago-id":
			a.Wago = val
		case "x-wowi-id":
			a.WoWI = val
		}
	}
	return a, sc.Err()
}

// Missing returns the folders in other that aren't in installed (compared case-insensitively), sorted.
func Missing(installed, other []Addon) []string {
	have := map[string]bool{}
	for _, a := range installed {
		have[strings.ToLower(a.Folder)] = true
	}
	var out []string
	for _, a := range other {
		if !have[strings.ToLower(a.Folder)] {
			out = append(out, a.Folder)
		}
	}
	sort.Strings(out)
	return out
}
