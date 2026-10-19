package syncdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultHydrateTimeout is how long to wait for a cloud placeholder file to download.
const DefaultHydrateTimeout = 60 * time.Second

var ErrNotHydrated = errors.New("file hasn't downloaded from the cloud yet")

// Hydrator reads a file from the sync folder, making the cloud client download it if it is only a
// placeholder (Google Drive streaming, OneDrive/iCloud files-on-demand).
type Hydrator interface {
	ReadFile(path string) ([]byte, error)
}

// TimeoutHydrator reads the whole file, which makes the cloud client fetch it. If that takes longer
// than Timeout it gives up with ErrNotHydrated. (The abandoned read finishes or fails on its own.)
type TimeoutHydrator struct {
	Timeout time.Duration
}

func (h TimeoutHydrator) ReadFile(path string) ([]byte, error) {
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := os.ReadFile(path)
		ch <- result{b, err}
	}()
	select {
	case r := <-ch:
		return r.b, r.err
	case <-time.After(h.Timeout):
		return nil, fmt.Errorf("%w: %s", ErrNotHydrated, filepath.Base(path))
	}
}
