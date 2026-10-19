package syncdir

import (
	"path/filepath"
	"slices"
)

// Candidates returns existing folders that look like a cloud client's synced root, for the setup
// prompt: OneDrive (from its env vars), Dropbox and iCloud under the profile, and Google Drive for
// Desktop's "My Drive" (in the profile or on a mounted drive letter). getenv and isDir are injected
// so tests don't depend on the real machine.
func Candidates(getenv func(string) string, isDir func(string) bool) []string {
	var out []string
	add := func(p string) {
		if p != "" && !slices.Contains(out, p) && isDir(p) {
			out = append(out, p)
		}
	}
	add(getenv("OneDrive"))
	add(getenv("OneDriveConsumer"))
	if home := getenv("USERPROFILE"); home != "" {
		add(filepath.Join(home, "Dropbox"))
		add(filepath.Join(home, "iCloudDrive"))
		add(filepath.Join(home, "My Drive"))
	}
	for l := 'D'; l <= 'Z'; l++ {
		add(string(l) + `:\My Drive`)
	}
	return out
}
