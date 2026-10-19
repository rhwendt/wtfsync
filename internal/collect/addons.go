package collect

import (
	"bufio"
	"cmp"
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

// MissingAddon is an addon another PC has that this one doesn't: its folders that are missing here
// and, when its .toc names a project, where to download it.
type MissingAddon struct {
	Name    string   // the addon's main folder
	Folders []string // sorted
	URL     string   // "" when no project id is known
}

// Missing returns the addons in other whose folders aren't in installed (compared
// case-insensitively), sorted by name. Folders that share a project id, or are named after another
// missing folder (ElvUI_Options next to ElvUI), are grouped as one addon.
func Missing(installed, other []Addon) []MissingAddon {
	have := map[string]bool{}
	for _, a := range installed {
		have[strings.ToLower(a.Folder)] = true
	}
	var missing []Addon
	for _, a := range other {
		if !have[strings.ToLower(a.Folder)] {
			missing = append(missing, a)
		}
	}
	// Shortest names first, so a main folder exists before the folders named after it.
	sort.Slice(missing, func(i, j int) bool {
		a, b := missing[i].Folder, missing[j].Folder
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
	var groups []*MissingAddon
	byID := map[string]*MissingAddon{}
	for _, a := range missing {
		g := byID[projectKey(a)]
		if g == nil || projectKey(a) == "" {
			g = namedAfter(groups, a.Folder)
		}
		if g == nil {
			g = &MissingAddon{Name: a.Folder}
			groups = append(groups, g)
		}
		g.Folders = append(g.Folders, a.Folder)
		if k := projectKey(a); k != "" && byID[k] == nil {
			byID[k] = g
		}
	}
	out := make([]MissingAddon, 0, len(groups))
	for _, g := range groups {
		sort.Strings(g.Folders)
		g.URL = groupURL(g.Folders, missing)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil
	}
	return out
}

// projectKey identifies an addon's project from the first id its .toc gives; "" if none.
func projectKey(a Addon) string {
	switch {
	case a.Curse != "":
		return "curse:" + a.Curse
	case a.Wago != "":
		return "wago:" + a.Wago
	case a.WoWI != "":
		return "wowi:" + a.WoWI
	}
	return ""
}

// namedAfter finds the group whose main folder this folder is named after (ElvUI for ElvUI_Options).
func namedAfter(groups []*MissingAddon, folder string) *MissingAddon {
	f := strings.ToLower(folder)
	for _, g := range groups {
		n := strings.ToLower(g.Name)
		if strings.HasPrefix(f, n+"_") || strings.HasPrefix(f, n+"-") {
			return g
		}
	}
	return nil
}

// groupURL links the group's project, preferring CurseForge, then Wago, then WoWInterface.
func groupURL(folders []string, addons []Addon) string {
	in := map[string]bool{}
	for _, f := range folders {
		in[f] = true
	}
	var curse, wago, wowi string
	for _, a := range addons {
		if !in[a.Folder] {
			continue
		}
		if curse == "" && isID(a.Curse) {
			curse = "https://www.curseforge.com/projects/" + a.Curse
		}
		if wago == "" && isID(a.Wago) {
			wago = "https://addons.wago.io/addons/" + a.Wago
		}
		if wowi == "" && isID(a.WoWI) {
			wowi = "https://www.wowinterface.com/downloads/info" + a.WoWI
		}
	}
	return cmp.Or(curse, wago, wowi)
}

// isID accepts plain alphanumeric ids, so text from a .toc can't turn into an odd link.
func isID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z') {
			return false
		}
	}
	return true
}
