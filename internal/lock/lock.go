// Package lock makes sure only one wtfsync process syncs at a time on this PC (CLI or tray).
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrHeld = errors.New("another wtfsync sync is running")

// Lock is held until Release, or until the process exits (the OS drops file locks then too).
type Lock struct {
	f *os.File
}

// Acquire takes the lock in dir (wtfsync's data folder) without waiting. It returns ErrHeld if
// another process, or another Acquire in this one, holds it.
func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "sync.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlockFile(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	l.f = nil
	return err
}
