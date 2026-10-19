// Package fsutil has small file helpers shared by the packages that write to disk.
package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path through a temp file in the same folder and a rename, creating
// parent folders, so readers (and cloud clients) never see a half-written file.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// WriteJSON writes v as indented JSON with WriteAtomic.
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(path, append(b, '\n'))
}

// IsDir reports whether path exists and is a folder.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
