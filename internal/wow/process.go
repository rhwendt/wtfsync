package wow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Processes reports whether a flavor's game client is running.
type Processes interface {
	Running(f Flavor) (bool, error)
}

// ProcessChecker matches running executables against a flavor's folder, so flavors that share an
// executable name (classic and classic_era both run WowClassic.exe) are told apart by path.
type ProcessChecker struct {
	// Full executable paths of running processes. A bare file name means the path couldn't be
	// read; a game executable listed that way counts as running for every flavor (fail closed).
	List func() ([]string, error)
}

func (c ProcessChecker) Running(f Flavor) (bool, error) {
	paths, err := c.List()
	if err != nil {
		return false, fmt.Errorf("list processes: %w", err)
	}
	dir := strings.ToLower(filepath.Clean(f.Dir)) + string(filepath.Separator)
	for _, p := range paths {
		lp := strings.ToLower(filepath.Clean(p))
		base := filepath.Base(lp)
		game := strings.HasPrefix(base, "wow") && strings.HasSuffix(base, ".exe")
		if game && (strings.HasPrefix(lp, dir) || base == lp) {
			return true, nil
		}
	}
	return false, nil
}

// System checks the real process list.
func System() ProcessChecker { return ProcessChecker{List: systemProcesses} }
