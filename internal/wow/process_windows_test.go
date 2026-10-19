//go:build windows

package wow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemProcessesIncludesSelf(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := systemProcesses()
	if err != nil {
		t.Fatal(err)
	}
	// Compare base names: the temp dir may appear as an 8.3 short path in one and long in the other.
	for _, p := range paths {
		if strings.EqualFold(filepath.Base(p), filepath.Base(self)) {
			return
		}
	}
	t.Fatalf("own process %s not in %d listed processes", filepath.Base(self), len(paths))
}
