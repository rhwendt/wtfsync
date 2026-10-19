package syncdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/rhwendt/wtfsync/internal/fsutil"
)

const timeLayout = "20060102T150405Z"

var nameRE = regexp.MustCompile(`^(\d{6,})_(\d{8}T\d{6}Z)_([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.tar\.gz$`)

// SnapshotName identifies a snapshot: <seq>_<utc>_<machine uuid>.tar.gz.
type SnapshotName struct {
	Seq  int
	Time time.Time
	UUID string
}

func (n SnapshotName) String() string {
	return fmt.Sprintf("%06d_%s_%s.tar.gz", n.Seq, n.Time.UTC().Format(timeLayout), n.UUID)
}

// Before orders snapshots by seq, then time, then UUID, so a PC with a wrong clock can't make an
// old snapshot look newest and two PCs that pushed the same seq still sort the same way everywhere.
func (n SnapshotName) Before(o SnapshotName) bool {
	if n.Seq != o.Seq {
		return n.Seq < o.Seq
	}
	if !n.Time.Equal(o.Time) {
		return n.Time.Before(o.Time)
	}
	return n.UUID < o.UUID
}

// ParseName parses a snapshot file name; anything else (temp files, cloud conflict copies) is rejected.
func ParseName(s string) (SnapshotName, bool) {
	m := nameRE.FindStringSubmatch(s)
	if m == nil {
		return SnapshotName{}, false
	}
	seq, err := strconv.Atoi(m[1])
	if err != nil {
		return SnapshotName{}, false
	}
	t, err := time.Parse(timeLayout, m[2])
	if err != nil {
		return SnapshotName{}, false
	}
	return SnapshotName{Seq: seq, Time: t, UUID: m[3]}, true
}

func (d *Dir) snapshotDir(flavor string) string {
	return filepath.Join(d.Root, flavor, "snapshots")
}

// Snapshots lists a flavor's snapshots, oldest first.
func (d *Dir) Snapshots(flavor string) ([]SnapshotName, error) {
	entries, err := os.ReadDir(d.snapshotDir(flavor))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SnapshotName
	for _, e := range entries {
		if n, ok := ParseName(e.Name()); ok && !e.IsDir() {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}

// Newest returns the newest snapshot of a flavor; ok is false if there are none.
func (d *Dir) Newest(flavor string) (n SnapshotName, ok bool, err error) {
	all, err := d.Snapshots(flavor)
	if err != nil || len(all) == 0 {
		return SnapshotName{}, false, err
	}
	return all[len(all)-1], true, nil
}

// ReadSnapshot reads a snapshot through the Hydrator.
func (d *Dir) ReadSnapshot(flavor string, n SnapshotName) ([]byte, error) {
	return d.Hydrator.ReadFile(filepath.Join(d.snapshotDir(flavor), n.String()))
}

// WriteSnapshot stores a new snapshot. Snapshots are immutable, so an existing name is an error.
func (d *Dir) WriteSnapshot(flavor string, n SnapshotName, data []byte) error {
	p := filepath.Join(d.snapshotDir(flavor), n.String())
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("snapshot %s already exists", n)
	}
	return fsutil.WriteAtomic(p, data)
}

// Prune deletes a flavor's oldest snapshots so that keep remain, skipping names in protect.
func (d *Dir) Prune(flavor string, keep int, protect map[string]bool) error {
	all, err := d.Snapshots(flavor)
	if err != nil {
		return err
	}
	for _, n := range all[:max(0, len(all)-keep)] {
		if protect[n.String()] {
			continue
		}
		err := os.Remove(filepath.Join(d.snapshotDir(flavor), n.String()))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
