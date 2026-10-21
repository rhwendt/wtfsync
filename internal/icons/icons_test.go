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
