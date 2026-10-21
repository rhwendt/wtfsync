package diag

import (
	"time"

	"golang.org/x/sys/windows"
)

// cpuTime is the CPU this process has used, user and kernel.
func cpuTime() time.Duration {
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(windows.CurrentProcess(), &created, &exited, &kernel, &user) != nil {
		return 0
	}
	ticks := func(f windows.Filetime) time.Duration { // 100 ns units
		return time.Duration(uint64(f.HighDateTime)<<32|uint64(f.LowDateTime)) * 100
	}
	return ticks(kernel) + ticks(user)
}
