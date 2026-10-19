// Package autostart turns "start wtfsync when Windows starts" on and off. It uses the same
// HKCU Run value the installer's checkbox writes.
package autostart

// Run is one value under HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
type Run struct {
	Name    string // "wtfsync"
	Command string // `"C:\...\wtfsync-tray.exe"`
}

// Setting is what the tray's settings window needs; tests fake it.
type Setting interface {
	Enabled() (bool, error)
	Set(on bool) error
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
