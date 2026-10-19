//go:build unix

package syncdir

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO with no writer blocks on open, standing in for a cloud placeholder that never downloads.
func TestTimeoutHydratorGivesUp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "placeholder")
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Skip("mkfifo:", err)
	}
	_, err := TimeoutHydrator{Timeout: 50 * time.Millisecond}.ReadFile(p)
	if !errors.Is(err, ErrNotHydrated) {
		t.Fatalf("err = %v, want ErrNotHydrated", err)
	}
}
