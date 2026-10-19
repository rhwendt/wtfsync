package icons

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIconsAreDistinct32pxPNGs(t *testing.T) {
	seen := map[string]bool{}
	for name, b := range map[string][]byte{"idle": Idle, "syncing": Syncing, "new": NewSettings, "attention": Attention} {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if s := img.Bounds().Size(); s.X != 32 || s.Y != 32 {
			t.Fatalf("%s: size %v", name, s)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("%s: corner should be transparent", name)
		}
		c := img.At(16, 16)
		r, g, b, _ := c.RGBA()
		key := string([]byte{byte(r), byte(g), byte(b)})
		if seen[key] {
			t.Fatalf("%s: same colour as another icon", name)
		}
		seen[key] = true
	}
}
