//go:build !windows

package wow

// systemProcesses is only meaningful on Windows; elsewhere nothing counts as running
// (wtfsync targets Windows; tests use fakes).
func systemProcesses() ([]string, error) { return nil, nil }
