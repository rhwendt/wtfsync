package ui

import (
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"

	"github.com/rhwendt/wtfsync/internal/icons"
)

type appIcon struct{ big, small w32.HICON }

// The taskbar and Alt+Tab icons, made once per colouring: Windows keeps using the handles.
var appIcons = sync.OnceValues(func() (onDark, onLight appIcon) {
	load := func(png []byte) appIcon {
		big, _ := w32.CreateLargeHIconFromImage(png)
		small, _ := w32.CreateSmallHIconFromImage(png)
		return appIcon{big, small}
	}
	return load(icons.AppOnDark), load(icons.AppOnLight)
})

var setWindowThemeAttribute = windows.NewLazySystemDLL("uxtheme.dll").NewProc("SetWindowThemeAttribute")

// setIcon gives a shown window the app icon that stands out on the taskbar (the white tile on a
// dark taskbar, the dark one on a light taskbar) and keeps it out of the title bar, which the
// page's top bar already brands. Run on the main thread, after Show.
func setIcon(w application.Window) {
	hwnd := w32.HWND(uintptr(w.NativeWindow()))
	if hwnd == 0 {
		return
	}
	// WTA_NONCLIENT with WTNCA_NODRAWICON: the title bar draws no icon; the taskbar still does.
	opts := struct{ flags, mask uint32 }{1, 1}
	setWindowThemeAttribute.Call(uintptr(hwnd), 1, uintptr(unsafe.Pointer(&opts)), unsafe.Sizeof(opts))
	icon, onLight := appIcons()
	if !w32.IsSystemCurrentlyDarkMode() {
		icon = onLight
	}
	if icon.big != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(icon.big))
	}
	if icon.small != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(icon.small))
	}
}
