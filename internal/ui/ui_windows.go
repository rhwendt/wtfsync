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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	u.tray = u.wails.SystemTray.New()
	u.render()
	return u.wails.Run()
}

// loop runs for the tray's lifetime (started by Bridge.ServiceStartup once Wails is up).
func (u *UI) loop(ctx context.Context) {
	u.notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: "pull", Actions: []notifications.NotificationAction{{ID: "pull", Title: "Pull"}}})
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
		u.open("wizard")
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
}

func (u *UI) logHealth(h *diag.Health) {
	u.opts.Log.Printf("health: %s windows=%d", h.Line(), len(u.wails.Window.GetAll()))
}

// do runs a menu or notification action. Sync work runs off the UI thread.
func (u *UI) do(id string) {
	u.opts.Log.Printf("menu: %s", id)
	switch id {
	case "push":
		go u.app.Push()
	case "pull":
		go u.app.Pull()
	case "update":
		go u.installUpdate()
	case "quit":
		go u.quit()
	case "setup":
		u.open("wizard")
	case "status", "restore", "addons", "settings":
		u.open(id)
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
	if u.app.View().SetUp {
		u.open("status")
	} else {
		u.open("wizard")
	}
}

type page struct {
	title         string
	width, height int
}

var pages = map[string]page{
	"wizard":    {"Set up", 560, 520},
	"conflicts": {"Settings changed on both PCs", 640, 480},
	"settings":  {"Settings", 560, 560},
	"status":    {"Status", 640, 420},
	"addons":    {"Missing addons", 600, 420},
	"restore":   {"Restore last backup", 600, 440},
}

// open shows a window, focusing it if it's already open. Each conflict question gets a window
// of its own, and closing it cancels only that question.
func (u *UI) open(name string) {
	if name == "conflicts" {
		u.openConflicts(u.prompt.request().ID)
		return
	}
	application.InvokeAsync(func() {
		if w, ok := u.wails.Window.GetByName(name); ok {
			w.Show()
			w.Focus()
			return
		}
		u.newWindow(name, pages[name], "/"+name+".html").Show()
	})
}

func (u *UI) openConflicts(id int) {
	application.InvokeAsync(func() {
		w := u.newWindow(fmt.Sprintf("conflicts-%d", id), pages["conflicts"], fmt.Sprintf("/conflicts.html?id=%d", id))
		w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.prompt.reply(id, nil, true) })
		w.Show()
	})
}

// newWindow makes one of the tray's windows, logging when it opens and closes. Its background
// follows the Windows theme, so a page that hasn't loaded (or failed to) isn't a black box.
func (u *UI) newWindow(name string, p page, url string) *application.WebviewWindow {
	bg := application.NewRGB(0xff, 0xff, 0xff)
	if u.wails.Env.IsDarkMode() {
		bg = application.NewRGB(0x20, 0x20, 0x20)
	}
	u.opts.Log.Printf("window: open %s", name)
	w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: name, Title: "wtfsync — " + p.title, Width: p.width, Height: p.height, URL: url, BackgroundColour: bg,
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
