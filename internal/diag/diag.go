// Package diag describes the tray's own health for its log: goroutines, memory and CPU time.
package diag

import (
	"fmt"
	"runtime"
	"time"
)

// Health writes health lines; it remembers the last CPU time so each line shows the CPU used since.
type Health struct {
	last time.Duration
}

// Line is one health line, e.g. "goroutines=12 heap=8MB sys=24MB cpu=3.1s (+0.2s)".
func (h *Health) Line() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cpu := cpuTime()
	since := cpu - h.last
	h.last = cpu
	return fmt.Sprintf("goroutines=%d heap=%dMB sys=%dMB cpu=%gs (+%gs)", runtime.NumGoroutine(),
		m.HeapAlloc>>20, m.Sys>>20, cpu.Round(100*time.Millisecond).Seconds(), since.Round(100*time.Millisecond).Seconds())
}
