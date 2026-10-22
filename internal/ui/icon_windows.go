package ui

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"

	"github.com/rhwendt/wtfsync/internal/icons"
)

// The taskbar and Alt+Tab icons, made once per colouring: Windows keeps using the handles.
var appIcons = sync.OnceValues(func() (onDark, onLight w32.HICON) {
	onDark, _ = w32.CreateLargeHIconFromImage(icons.AppOnDark)
	onLight, _ = w32.CreateLargeHIconFromImage(icons.AppOnLight)
	return onDark, onLight
})

// blankIcon is a fully transparent small icon: the title bar draws it (Windows draws the small
// icon there whatever the frame style or theme says), so the title bar shows nothing.
var blankIcon = sync.OnceValue(func() w32.HICON {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	icon, _ := w32.CreateSmallHIconFromImage(buf.Bytes())
	return icon
})

// setIcon gives a shown window the app icon that stands out on the taskbar (the white tile on a
// dark taskbar, the dark one on a light taskbar) and a blank one for the title bar, which the
// page's top bar already brands. Run on the main thread, after Show.
func setIcon(w application.Window) {
	hwnd := w32.HWND(uintptr(w.NativeWindow()))
	if hwnd == 0 {
		return
	}
	icon, onLight := appIcons()
	if !w32.IsSystemCurrentlyDarkMode() {
		icon = onLight
	}
	if icon != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(icon))
	}
	if blank := blankIcon(); blank != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(blank))
	}
}
