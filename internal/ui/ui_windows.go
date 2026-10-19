//go:build windows

// Package ui is the tray app's Wails glue: it renders package app's View as a tray icon and
// menu, opens the windows, shows notifications, and binds Bridge for the pages in web/. It holds
// no sync logic.
package ui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"golang.org/x/sys/windows"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/icons"
)

//go:embed web
var webFiles embed.FS

// Options are what the windows need beyond app.Deps.
type Options struct {
	InstallDirs []string               // where to look for WoW (wow.DefaultInstallDirs)
	Getenv      func(string) string    // for cloud folder detection
	Hostname    func() (string, error) // default PC name
	Log         *log.Logger
}

const uniqueID = "wtfsync-tray-6f2b7c1e"

type UI struct {
	opts   Options
	app    *app.App
	wails  *application.App
	tray   *application.SystemTray
	notes  *notifications.NotificationService
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
	u.notes = notifications.New()
	web, err := fs.Sub(webFiles, "web")
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
	tick, daily := time.NewTicker(time.Minute), time.NewTicker(24*time.Hour)
	defer tick.Stop()
	defer daily.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			u.app.Tick()
		case <-daily.C:
			u.app.CheckUpdate(ctx)
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

// do runs a menu or notification action. Sync work runs off the UI thread.
func (u *UI) do(id string) {
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

func (u *UI) notify(n app.Notice) {
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
		p := pages[name]
		w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: name, Title: "wtfsync — " + p.title, Width: p.width, Height: p.height, URL: "/" + name + ".html",
		})
		w.Show()
	})
}

func (u *UI) openConflicts(id int) {
	application.InvokeAsync(func() {
		p := pages["conflicts"]
		w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: fmt.Sprintf("conflicts-%d", id), Title: "wtfsync — " + p.title, Width: p.width, Height: p.height,
			URL: fmt.Sprintf("/conflicts.html?id=%d", id),
		})
		w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.prompt.reply(id, nil, true) })
		w.Show()
	})
}
