// Package icons draws wtfsync's icons from the SVGs beside it: the tray's dark "WTF" tile with a
// strip along the bottom whose colour and symbol are the sync state, and the app icon: the top
// bar's WTF logo, in two colourings so it stands out on dark and light backgrounds.
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

	AppOnDark  = renderAt("app-on-dark.svg", appSize)  // white tile: dark taskbar, and the exe's icon
	AppOnLight = renderAt("app-on-light.svg", appSize) // dark tile: light taskbar
)

// size is what the tray PNGs are drawn at; Windows scales them down to the tray's 16–32 px.
// appSize is the app icon's, which Windows also shows large (Alt+Tab, the taskbar).
const (
	size    = 64
	appSize = 256
)

func render(name string) []byte { return renderAt(name, size) }

func renderAt(name string, size int) []byte {
	f, err := svgs.Open(name)
	if err != nil {
		panic(err) // embedded above: only a typo here gets this far
	}
	defer f.Close()
	icon, err := oksvg.ReadIconStream(f, oksvg.StrictErrorMode)
	if err != nil {
		panic(name + ": " + err.Error()) // the tests render every icon
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding an in-memory RGBA image can't fail
	}
	return buf.Bytes()
}
