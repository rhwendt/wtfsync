//go:build !windows

package autostart

import "testing"

func TestUnsupportedOffWindows(t *testing.T) {
	r := Run{Name: "wtfsync"}
	if on, err := r.Enabled(); on || err != nil {
		t.Fatalf("Enabled = %v, %v", on, err)
	}
	if err := r.Set(true); err == nil {
		t.Fatal("Set(true) should fail off Windows")
	}
}
