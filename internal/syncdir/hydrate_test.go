package syncdir

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTimeoutHydratorReadsFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	os.WriteFile(p, []byte("data"), 0o644)
	b, err := TimeoutHydrator{Timeout: time.Second}.ReadFile(p)
	if err != nil || string(b) != "data" {
		t.Fatalf("got %q, %v", b, err)
	}
}
