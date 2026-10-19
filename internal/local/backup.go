package local

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rhwendt/wtfsync/internal/fsutil"
)

const (
	keepBackups  = 10
	backupLayout = "20060102T150405.000Z"
)

type backupIndex struct {
	Present []string `json:"present"` // backed-up files, under files/
	Absent  []string `json:"absent"`  // paths that didn't exist locally at backup time
}

func (s Store) backupDir(flavor, id string) string {
	return filepath.Join(s.Dir, "backups", flavor, id)
}

// Backup saves the current local version of each WTF-relative path into a new backup set, noting
// paths that don't exist, and returns the set id. Only the newest 10 sets per flavor are kept.
func (s Store) Backup(flavor, wtfDir string, paths []string, now time.Time) (string, error) {
	id := now.UTC().Format(backupLayout)
	dir := s.backupDir(flavor, id)
	var idx backupIndex
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(wtfDir, filepath.FromSlash(p)))
		if errors.Is(err, fs.ErrNotExist) {
			idx.Absent = append(idx.Absent, p)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("back up %s: %w", p, err)
		}
		if err := fsutil.WriteAtomic(filepath.Join(dir, "files", filepath.FromSlash(p)), b); err != nil {
			return "", err
		}
		idx.Present = append(idx.Present, p)
	}
	if err := fsutil.WriteJSON(filepath.Join(dir, "index.json"), idx); err != nil {
		return "", err
	}
	return id, s.pruneBackups(flavor)
}

// BackupSets lists a flavor's backup set ids, newest first.
func (s Store) BackupSets(flavor string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.Dir, "backups", flavor))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.backupDir(flavor, e.Name()), "index.json")); err == nil {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

func (s Store) pruneBackups(flavor string) error {
	ids, err := s.BackupSets(flavor)
	if err != nil {
		return err
	}
	for _, id := range ids[min(len(ids), keepBackups):] {
		if err := os.RemoveAll(s.backupDir(flavor, id)); err != nil {
			return err
		}
	}
	return nil
}

// Restore puts a backup set back into wtfDir: backed-up files are rewritten and files that were
// absent at backup time are deleted.
func (s Store) Restore(flavor, id, wtfDir string) error {
	idx, err := s.readIndex(flavor, id)
	if err != nil {
		return err
	}
	dir := s.backupDir(flavor, id)
	for _, p := range idx.Present {
		b, err := os.ReadFile(filepath.Join(dir, "files", filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		if err := fsutil.WriteAtomic(filepath.Join(wtfDir, filepath.FromSlash(p)), b); err != nil {
			return err
		}
	}
	for _, p := range idx.Absent {
		err := os.Remove(filepath.Join(wtfDir, filepath.FromSlash(p)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// BackupPaths lists the WTF-relative paths a backup set covers: the files it rewrites and the ones
// it deletes on restore.
func (s Store) BackupPaths(flavor, id string) ([]string, error) {
	idx, err := s.readIndex(flavor, id)
	if err != nil {
		return nil, err
	}
	paths := append(append([]string{}, idx.Present...), idx.Absent...)
	sort.Strings(paths)
	return paths, nil
}

func (s Store) readIndex(flavor, id string) (backupIndex, error) {
	var idx backupIndex
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id {
		return idx, fmt.Errorf("invalid backup id %q", id)
	}
	if err := readJSON(filepath.Join(s.backupDir(flavor, id), "index.json"), &idx); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return idx, fmt.Errorf("no backup %q for %s", id, flavor)
		}
		return idx, err
	}
	return idx, nil
}
