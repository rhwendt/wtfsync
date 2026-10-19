// Package collect gathers the SavedVariables files wtfsync syncs and the list of installed addons.
package collect

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Collect reads every synced SavedVariables file under a flavor's WTF folder, keyed by
// slash-separated path relative to WTF (e.g. "Account/123#1/SavedVariables/ElvUI.lua").
// A missing WTF/Account folder yields an empty map.
func Collect(wtfDir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	acct := filepath.Join(wtfDir, "Account")
	err := filepath.WalkDir(acct, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == acct && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(wtfDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !Synced(rel) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = b
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect %s: %w", wtfDir, err)
	}
	return files, nil
}

// Synced reports whether a WTF-relative slash path is one wtfsync syncs. It also rejects any path
// that could escape the WTF folder, so it doubles as the safety check for paths read from snapshots.
func Synced(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `\:`) {
			return false
		}
	}
	if parts[0] != "Account" || !strings.EqualFold(path.Ext(rel), ".lua") {
		return false
	}
	switch len(parts) {
	case 4: // Account/<acct>/SavedVariables/<file>
		return strings.EqualFold(parts[2], "SavedVariables")
	case 6: // Account/<acct>/<realm>/<char>/SavedVariables/<file>
		return strings.EqualFold(parts[4], "SavedVariables")
	}
	return false
}

// Hash returns "sha256:<hex>" for file contents.
func Hash(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

// Hashes maps each path to the Hash of its contents.
func Hashes(files map[string][]byte) map[string]string {
	h := make(map[string]string, len(files))
	for p, b := range files {
		h[p] = Hash(b)
	}
	return h
}
