package diag

import (
	"regexp"
	"testing"
)

func TestHealthLineHasTheNumbersALockupNeeds(t *testing.T) {
	var h Health
	line := h.Line()
	re := regexp.MustCompile(`^goroutines=\d+ heap=\d+MB sys=\d+MB cpu=\d+(\.\d+)?s \(\+\d+(\.\d+)?s\)$`)
	if !re.MatchString(line) {
		t.Fatalf("line %q", line)
	}
}

func TestCPUSinceLastLineGrowsWithWork(t *testing.T) {
	var h Health
	h.Line()
	start := h.last
	x := 0
	for i := range 200_000_000 {
		x += i % 7
	}
	_ = x
	h.Line()
	if h.last <= start {
		t.Fatalf("cpu time didn't move: %v → %v", start, h.last)
	}
}
