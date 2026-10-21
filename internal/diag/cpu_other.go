//go:build !windows

package diag

import (
	"syscall"
	"time"
)

// cpuTime is the CPU this process has used, user and kernel.
func cpuTime() time.Duration {
	var u syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &u) != nil {
		return 0
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}
