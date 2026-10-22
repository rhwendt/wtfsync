//go:build windows

// Package ui is the tray app's Wails glue: it renders package app's View as a tray icon and
// menu, opens the windows, shows notifications, and binds Bridge for the pages in web/. It holds
// no sync logic.
package ui

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"golang.org/x/sys/windows"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/diag"
	"github.com/rhwendt/wtfsync/internal/icons"
)

// Options are what the windows need beyond app.Deps.
type Options struct {
	InstallDirs []string               // where to look for WoW (wow.DefaultInstallDirs)
	Getenv      func(string) string    // for cloud folder detection
	Hostname    func() (string, error) // default PC name
	Log         *log.Logger
	LogDir      string // holds tray.log and crash.log; the menu's Open log folder opens it
}

const uniqueID = "wtfsync-tray-6f2b7c1e"

type UI struct {
	opts   Options
	app    *app.App
	wails  *application.App
	tray   *application.SystemTray
	notes  *notes
	prompt *prompter

	quitting atomic.Bool // set by quit, so closing the main window then really closes it
}

// Run starts the tray and blocks until it quits. With --quit in os.Args it asks a running tray
// to quit instead (Wails' single-instance check hands the arguments over and exits this process);
// if none is running there's nothing to do.
func Run(d app.Deps, o Options) error {
	u := &UI{opts: o}
	u.prompt = &prompter{open: u.open}
	d.Prompt, d.Notify, d.Changed, d.Log = u.prompt, u.notify, u.render, o.Log
	u.app = app.New(d)
	u.notes = &notes{NotificationService: notifications.New(), log: o.Log}
	web, err := assets()
	if err != nil {
		return err
	}
	u.wails = application.New(application.Options{
		Name:        "wtfsync",
		Description: "Syncs World of Warcraft addon settings between your PCs",
		Icon:        icons.AppOnDark, // the taskbar's until setIcon picks one for the Windows theme
		Services:    []application.Service{application.NewService(u.notes), application.NewService(&Bridge{u: u})},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(web)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               uniqueID,
			OnSecondInstanceLaunch: u.secondInstance,
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true}, // the tray keeps running
		// Wails' own warnings and errors (e.g. a page it can't serve) go to tray.log too.
		Logger:   slog.New(slog.NewTextHandler(o.Log.Writer(), &slog.HandlerOptions{Level: slog.LevelWarn})),
		LogLevel: slog.LevelWarn,
		PanicHandler: func(p *application.PanicDetails) {
			o.Log.Printf("panic: %v\n%s", p.Error, p.FullStackTrace)
		},
	})
	if slices.Contains(os.Args[1:], "--quit") {
		return nil // no other tray was running
	}
	u.wails.Event.OnApplicationEvent(events.Windows.SystemThemeChanged, func(*application.ApplicationEvent) {
		application.InvokeAsync(func() {
			for _, w := range u.wails.Window.GetAll() {
				setIcon(w)
			}
		})
	})
	u.tray = u.wails.SystemTray.New()
	u.tray.OnClick(func() { u.showMain("") }) // right-click still shows the menu
	u.render()
	return u.wails.Run()
}

// loop runs for the tray's lifetime (started by Bridge.ServiceStartup once Wails is up).
func (u *UI) loop(ctx context.Context) {
	u.notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: "sync", Actions: []notifications.NotificationAction{{ID: "sync", Title: "Sync"}}})
	u.notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: "addons", Actions: []notifications.NotificationAction{{ID: "addons", Title: "Show"}}})
	u.notes.OnNotificationResponse(func(r notifications.NotificationResult) {
		if r.Error != nil {
			return
		}
		action := r.Response.ActionIdentifier
		if action == notifications.DefaultActionIdentifier {
			action = r.Response.CategoryID
		}
		u.do(action)
	})
	u.app.Refresh()
	if !u.app.View().SetUp {
		u.showMain("wizard")
	}
	u.app.CheckUpdate(ctx)
	var h diag.Health
	u.logHealth(&h)
	tick, daily, health := time.NewTicker(time.Minute), time.NewTicker(24*time.Hour), time.NewTicker(10*time.Minute)
	defer tick.Stop()
	defer daily.Stop()
	defer health.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			u.app.Tick()
		case <-daily.C:
			u.app.CheckUpdate(ctx)
		case <-health.C:
			u.logHealth(&h)
		}
	}
}

// render shows the current View: icon, tooltip and menu. Safe from any goroutine.
func (u *UI) render() {
	if u.tray == nil {
		return
	}
	v := u.app.View()
	u.tray.SetIcon(map[app.IconState][]byte{
		app.Idle: icons.Idle, app.NewSettings: icons.NewSettings, app.Attention: icons.Attention, app.Syncing: icons.Syncing,
	}[v.Icon])
	u.tray.SetTooltip(v.Tooltip)
	menu := u.wails.NewMenu()
	for _, it := range app.Menu(v) {
		if it.ID == "-" {
			menu.AddSeparator()
			continue
		}
		id := it.ID
		menu.Add(it.Label).SetEnabled(it.Enabled).OnClick(func(*application.Context) { u.do(id) })
	}
	u.tray.SetMenu(menu)
	u.wails.Event.Emit("changed") // the main window reloads its data
}

func (u *UI) logHealth(h *diag.Health) {
	u.opts.Log.Printf("health: %s windows=%d", h.Line(), len(u.wails.Window.GetAll()))
}

// do runs a menu or notification action. Sync work runs off the UI thread.
func (u *UI) do(id string) {
	u.opts.Log.Printf("menu: %s", id)
	switch id {
	case "sync":
		go u.app.Sync()
	case "update":
		go u.installUpdate()
	case "quit":
		go u.quit()
	case "open":
		u.showMain("")
	case "addons":
		u.showMain("flavor:" + app.FirstMissing(u.app.View()))
	case "setup":
		u.showMain("wizard")
	case "settings":
		u.showMain("settings")
	case "logs":
		u.openLogFolder()
	}
}

func (u *UI) installUpdate() {
	dir := filepath.Join(os.TempDir(), "wtfsync-update")
	err := u.app.InstallUpdate(context.Background(), dir, func(installer string) error {
		cmd := exec.Command(installer, "/S")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
		return cmd.Start()
	})
	if err == nil {
		u.quit() // the installer replaces the files and starts the new tray
	}
}

// quit cancels any open conflict question, gives a running sync up to 8 s to finish so it isn't
// stopped halfway through writing files (the installer force-closes the tray after 10 s), and
// quits.
func (u *UI) quit() {
	u.quitting.Store(true)
	u.prompt.shutdown()
	if !u.app.Close(8*time.Second) && u.opts.Log != nil {
		u.opts.Log.Print("quitting while a sync is still running")
	}
	u.wails.Quit()
}

// notes is Wails' notifications service, except that a failed startup (registering the app for
// toasts under HKCU) is logged instead of stopping the tray; notifications are then skipped.
type notes struct {
	*notifications.NotificationService
	log *log.Logger
	ok  atomic.Bool
}

func (n *notes) ServiceStartup(ctx context.Context, o application.ServiceOptions) error {
	if err := n.NotificationService.ServiceStartup(ctx, o); err != nil {
		if n.log != nil {
			n.log.Printf("notifications off: %v", err)
		}
		return nil
	}
	n.ok.Store(true)
	return nil
}

func (u *UI) notify(n app.Notice) {
	if !u.notes.ok.Load() {
		return // startup failed and was logged
	}
	opts := notifications.NotificationOptions{ID: n.ID, Title: n.Title, Body: n.Body, CategoryID: n.Action}
	var err error
	if n.Action != "" {
		err = u.notes.SendNotificationWithActions(opts)
	} else {
		err = u.notes.SendNotification(opts)
	}
	if err != nil && u.opts.Log != nil {
		u.opts.Log.Printf("notify %s: %v", n.ID, err)
	}
}

func (u *UI) secondInstance(data application.SecondInstanceData) {
	if slices.Contains(data.Args, "--quit") {
		go u.quit()
		return
	}
	u.showMain("")
}

// open is how the conflict prompter asks for its dialog.
func (u *UI) open(name string) {
	if name == "conflicts" {
		u.openConflicts(u.prompt.request().ID)
	}
}

// showMain opens the main window, or brings it to the front, on view ("" keeps what it shows;
// "settings", "wizard", or "flavor:<name>"). Closing it only hides it. Each time it's shown,
// status is refreshed.
func (u *UI) showMain(view string) {
	go u.app.Refresh()
	application.InvokeAsync(func() {
		if w, ok := u.wails.Window.GetByName("main"); ok {
			u.wails.Event.Emit("show", view)
			w.Show()
			w.Focus()
			return
		}
		addr := "/main.html"
		if name, ok := strings.CutPrefix(view, "flavor:"); ok {
			addr += "?flavor=" + url.QueryEscape(name)
		} else if view != "" {
			addr += "?view=" + view
		}
		w := u.newWindow("main", "", 860, 560, addr) // no title: the page's top bar has the logo
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if u.quitting.Load() {
				return
			}
			e.Cancel()
			w.Hide()
		})
		w.Show()
		setIcon(w)
	})
}

// openConflicts opens one conflict question's dialog; closing it cancels only that question.
func (u *UI) openConflicts(id int) {
	application.InvokeAsync(func() {
		w := u.newWindow(fmt.Sprintf("conflicts-%d", id), "Settings changed on both PCs", 680, 480, fmt.Sprintf("/conflicts.html?id=%d", id))
		w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.prompt.reply(id, nil, true) })
		w.Show()
		setIcon(w)
	})
}

// newWindow makes one of the tray's windows, logging when it opens and closes. Its background
// matches the appearance setting, so a page that hasn't loaded (or failed to) isn't a black box.
// Its icons (none in the title bar, the taskbar's by theme) are set by setIcon once it's shown.
func (u *UI) newWindow(name, title string, width, height int, url string) *application.WebviewWindow {
	s, _, _ := u.app.Settings()
	theme := s.Theme()
	bg := application.NewRGB(0xff, 0xff, 0xff)
	if theme == "dark" || (theme == "system" && u.wails.Env.IsDarkMode()) {
		bg = application.NewRGB(0x1f, 0x1f, 0x1f)
	}
	u.opts.Log.Printf("window: open %s", name)
	w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: name, Title: title, Width: width, Height: height, MinWidth: 640, MinHeight: 420, URL: url, BackgroundColour: bg,
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.opts.Log.Printf("window: closed %s", name) })
	return w
}

// openLogFolder shows tray.log and crash.log in Explorer.
func (u *UI) openLogFolder() {
	dir, err := windows.UTF16PtrFromString(u.opts.LogDir)
	if err == nil {
		err = windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), dir, nil, nil, windows.SW_SHOWNORMAL)
	}
	if err != nil {
		u.opts.Log.Printf("open log folder: %v", err)
	}
}
