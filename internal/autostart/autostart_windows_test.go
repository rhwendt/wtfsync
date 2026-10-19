//go:build windows

package autostart

import "testing"

func TestSetAndClearRunValue(t *testing.T) {
	r := Run{Name: "wtfsync-test-autostart", Command: `"C:\nowhere\wtfsync-tray.exe"`}
	t.Cleanup(func() { r.Set(false) })
	for _, on := range []bool{true, false, false} { // clearing twice is fine
		if err := r.Set(on); err != nil {
			t.Fatal(err)
		}
		if got, err := r.Enabled(); err != nil || got != on {
			t.Fatalf("after Set(%v): Enabled = %v, %v", on, got, err)
		}
	}
}
