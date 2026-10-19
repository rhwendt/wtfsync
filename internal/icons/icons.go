// Package icons draws the tray icons: a filled circle whose colour is the sync state.
package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	Idle        = circle(color.RGBA{0x2e, 0x9e, 0x5b, 0xff}) // green: in sync
	Syncing     = circle(color.RGBA{0x2f, 0x7f, 0xd8, 0xff}) // blue: working
	NewSettings = circle(color.RGBA{0xe8, 0xa3, 0x17, 0xff}) // amber: another PC has newer settings
	Attention   = circle(color.RGBA{0xd6, 0x3b, 0x3b, 0xff}) // red: needs you
)

const size = 32

func circle(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	const r = size/2 - 1
	for y := range size {
		for x := range size {
			dx, dy := x-size/2, y-size/2
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding an in-memory RGBA image can't fail
	}
	return buf.Bytes()
}
