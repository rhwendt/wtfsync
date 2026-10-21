// Package icons draws the tray icons from the SVGs beside it: a dark "WTF" tile with a strip
// along the bottom whose colour and symbol are the sync state.
package icons

import (
	"bytes"
	"embed"
	"image"
	"image/png"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed *.svg
var svgs embed.FS

var (
	Idle        = render("idle.svg")         // green tick: in sync
	Syncing     = render("syncing.svg")      // blue arrows: working
	NewSettings = render("new-settings.svg") // amber down arrow: another PC has newer settings
	Attention   = render("attention.svg")    // red exclamation mark: needs you
)

// size is what the PNGs are drawn at; Windows scales them down to the tray's 16–32 px.
const size = 64

func render(name string) []byte {
	f, err := svgs.Open(name)
	if err != nil {
		panic(err) // embedded above: only a typo here gets this far
	}
	defer f.Close()
	icon, err := oksvg.ReadIconStream(f, oksvg.StrictErrorMode)
	if err != nil {
		panic(name + ": " + err.Error()) // the tests render every icon
	}
	icon.SetTarget(0, 0, size, size)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding an in-memory RGBA image can't fail
	}
	return buf.Bytes()
}
