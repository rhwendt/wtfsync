package engine

import (
	"bytes"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// mergeBase finds the base for merging snapshot n (manifest m) into this PC. Normally that's this
// PC's last-synced files, but only if n descends from them. When two PCs pushed without seeing each
// other (same seq), n is a sibling of this PC's base, and the right base is the snapshot both grew
// from. known is false when no common snapshot can be found (pruned, or a different sync folder):
// then every difference is a conflict.
func mergeBase(d *syncdir.Dir, flavor string, st local.FlavorState, n syncdir.SnapshotName, m archive.Manifest) (base map[string]string, known bool) {
	if st.Base == "" {
		return nil, true // first link: no history to compare; resolve asks once
	}
	// n's ancestors, newest first.
	theirs := map[string]map[string]string{n.String(): m.Files}
	for cur := m.Base; cur != "" && theirs[cur] == nil; {
		if cur == st.Base {
			return st.Files, true
		}
		pm, ok := readManifest(d, flavor, cur)
		if !ok {
			break
		}
		theirs[cur] = pm.Files
		cur = pm.Base
	}
	// This PC's ancestors: the first one n also descends from is the common base.
	seen := map[string]bool{}
	for cur := st.Base; cur != "" && !seen[cur]; {
		if files, ok := theirs[cur]; ok {
			return files, true
		}
		seen[cur] = true
		pm, ok := readManifest(d, flavor, cur)
		if !ok {
			break
		}
		cur = pm.Base
	}
	return nil, false
}

func readManifest(d *syncdir.Dir, flavor, name string) (archive.Manifest, bool) {
	n, ok := syncdir.ParseName(name)
	if !ok {
		return archive.Manifest{}, false
	}
	data, err := d.ReadSnapshot(flavor, n)
	if err != nil {
		return archive.Manifest{}, false
	}
	m, _, err := archive.Read(bytes.NewReader(data))
	if err != nil {
		return archive.Manifest{}, false
	}
	return m, true
}
