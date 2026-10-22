package ui

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"

	"github.com/rhwendt/wtfsync/internal/icons"
)

// The taskbar and Alt+Tab icon, made once per colouring: Windows keeps using the handle.
var appIcons = sync.OnceValues(func() (onDark, onLight w32.HICON) {
	onDark, _ = w32.CreateLargeHIconFromImage(icons.AppOnDark)
	onLight, _ = w32.CreateLargeHIconFromImage(icons.AppOnLight)
	return onDark, onLight
})

// setIcon gives a shown window the app icon that stands out on the taskbar: the white tile on a
// dark taskbar, the dark one on a light taskbar. The title bar has none (DisableIcon), so the
// small icon is cleared. Run on the main thread, after Show.
func setIcon(w application.Window) {
	hwnd := w32.HWND(uintptr(w.NativeWindow()))
	if hwnd == 0 {
		return
	}
	icon, onLight := appIcons()
	if !w32.IsSystemCurrentlyDarkMode() {
		icon = onLight
	}
	if icon == 0 {
		return // the exe's icon (the white tile) stays
	}
	w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, 0)
	w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(icon))
}
