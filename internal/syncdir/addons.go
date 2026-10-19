package syncdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/fsutil"
)

var addonListRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.json$`)

// AddonList is the addons one PC has installed for a flavor, as of its last push. It's kept apart
// from snapshots so other PCs still see it after someone else pushes a newer snapshot.
type AddonList struct {
	UUID    string          `json:"uuid"`
	Name    string          `json:"name"`
	Updated time.Time       `json:"updated"`
	Addons  []collect.Addon `json:"addons"`
}

func (d *Dir) addonsDir(flavor string) string {
	return filepath.Join(d.Root, flavor, "addons")
}

// WriteAddons writes this PC's addon list for flavor. Each PC only ever writes its own file.
func (d *Dir) WriteAddons(flavor string, l AddonList) error {
	l.Updated = l.Updated.UTC()
	return fsutil.WriteJSON(filepath.Join(d.addonsDir(flavor), l.UUID+".json"), l)
}

// AddonLists reads every PC's addon list for flavor, sorted by UUID. Files that aren't
// <uuid>.json (cloud conflict copies, temp files) are ignored.
func (d *Dir) AddonLists(flavor string) ([]AddonList, error) {
	entries, err := os.ReadDir(d.addonsDir(flavor))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []AddonList
	for _, e := range entries {
		if e.IsDir() || !addonListRE.MatchString(e.Name()) {
			continue
		}
		var l AddonList
		if err := d.readJSON(filepath.Join(d.addonsDir(flavor), e.Name()), &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UUID < out[j].UUID })
	return out, nil
}
