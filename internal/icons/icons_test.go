package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func decode(t *testing.T, name string, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if s := img.Bounds().Size(); s.X != size || s.Y != size {
		t.Fatalf("%s: size %v, want %dpx", name, s, size)
	}
	return img
}

func rgb(c color.Color) [3]uint8 {
	r, g, b, _ := c.RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// Every icon is the same dark WTF tile; the strip along the bottom is the state's colour.
func TestIconsAreTheWTFTileWithAStateStrip(t *testing.T) {
	tile := [3]uint8{0x1e, 0x25, 0x30}
	for name, tt := range map[string]struct {
		png   []byte
		strip [3]uint8
	}{
		"idle":      {Idle, [3]uint8{0x2e, 0x9e, 0x5b}},
		"syncing":   {Syncing, [3]uint8{0x2f, 0x7f, 0xd8}},
		"new":       {NewSettings, [3]uint8{0xe8, 0xa3, 0x17}},
		"attention": {Attention, [3]uint8{0xd6, 0x3b, 0x3b}},
	} {
		img := decode(t, name, tt.png)
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("%s: rounded corner should be transparent", name)
		}
		if got := rgb(img.At(2, 32)); got != tile {
			t.Errorf("%s: tile colour %x, want %x", name, got, tile)
		}
		if got := rgb(img.At(10, 47)); got != tt.strip {
			t.Errorf("%s: strip colour %x, want %x", name, got, tt.strip)
		}
		if got := rgb(img.At(37, 20)); got != [3]uint8{0xff, 0xff, 0xff} { // the T's stem
			t.Errorf("%s: no white lettering at the T, got %x", name, got)
		}
	}
}

// The app icon is the top bar's WTF logo, drawn large: a white tile with dark letters for dark
// backgrounds (OnDark), and the same reversed for light ones (OnLight).
func TestAppIconsAreTheLogoAndItsReverse(t *testing.T) {
	white, dark := [3]uint8{0xff, 0xff, 0xff}, [3]uint8{0x1e, 0x25, 0x30}
	for name, tt := range map[string]struct {
		png             []byte
		tile, lettering [3]uint8
	}{
		"OnDark":  {AppOnDark, white, dark},
		"OnLight": {AppOnLight, dark, white},
	} {
		img, err := png.Decode(bytes.NewReader(tt.png))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if s := img.Bounds().Size(); s.X != appSize || s.Y != appSize {
			t.Fatalf("%s: size %v, want %dpx", name, s, appSize)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("%s: rounded corner should be transparent", name)
		}
		if got := rgb(img.At(appSize/2, appSize/4)); got != tt.tile {
			t.Errorf("%s: tile colour %x above the letters, want %x", name, got, tt.tile)
		}
		// The T's stem, x 143–160 of 256: solid well off its centre line.
		for _, x := range []int{146, 151, 156} {
			if got := rgb(img.At(x, appSize/2+10)); got != tt.lettering {
				t.Errorf("%s: lettering at x=%d on the T is %x, want %x", name, x, got, tt.lettering)
			}
		}
	}
}
