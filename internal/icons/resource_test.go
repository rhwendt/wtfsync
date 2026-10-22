package icons

import (
	"bytes"
	"testing"

	"github.com/tc-hib/winres"
)

// The exe carries the app icon at every size Explorer, the taskbar and Alt+Tab ask for, under the
// ID Wails loads for its window class, so it's also what a window shows before Options.Icon applies.
func TestResourceHasTheAppIconAtEverySize(t *testing.T) {
	var obj bytes.Buffer
	if err := Resource(&obj); err != nil {
		t.Fatal(err)
	}
	if b := obj.Bytes(); len(b) < 2 || b[0] != 0x64 || b[1] != 0x86 {
		t.Fatalf("not an amd64 COFF object: % x", obj.Bytes()[:min(obj.Len(), 4)])
	}
	rs, err := resources()
	if err != nil {
		t.Fatal(err)
	}
	icon, err := rs.GetIcon(winres.ID(appIconID))
	if err != nil {
		t.Fatalf("no icon %d: %v", appIconID, err)
	}
	var ico bytes.Buffer
	if err := icon.SaveICO(&ico); err != nil {
		t.Fatal(err)
	}
	// ICONDIR: reserved, type 1, count; then one 16-byte entry per image, width first (0 = 256).
	b := ico.Bytes()
	got := map[int]bool{}
	for i := range int(b[4]) {
		w := int(b[6+16*i])
		if w == 0 {
			w = 256
		}
		got[w] = true
	}
	for _, s := range []int{16, 24, 32, 48, 64, 256} {
		if !got[s] {
			t.Errorf("no %dpx image; have %v", s, got)
		}
	}
}
