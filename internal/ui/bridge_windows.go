//go:build windows

package ui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// Bridge is the Go side of the pages: each exported method is callable from JavaScript as
// "github.com/rhwendt/wtfsync/internal/ui.Bridge.<Method>" (see web/app.js).
type Bridge struct {
	u *UI
}

func (b *Bridge) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go b.u.loop(ctx)
	return nil
}

// WizardDefaults pre-fills the setup wizard.
type WizardDefaults struct {
	Installs []string // WoW installs found by the drive scan
	Clouds   []string // detected cloud folders
	WoWDir   string
	Sync     string
	Name     string
	Rerun    bool
}

func (b *Bridge) Defaults() WizardDefaults {
	o := b.u.opts
	d := WizardDefaults{Installs: list(setup.FindInstalls(o.InstallDirs)), Clouds: list(syncdir.Candidates(o.Getenv, fsutil.IsDir))}
	if s, ok, _ := b.u.app.Settings(); ok {
		d.WoWDir, d.Sync, d.Name, d.Rerun = s.WoWDir, s.SyncFolder, s.MachineName, true
	} else {
		d.Name, _ = o.Hostname()
		if len(d.Installs) > 0 {
			d.WoWDir = d.Installs[0]
		}
		if len(d.Clouds) > 0 {
			d.Sync = d.Clouds[0]
		}
	}
	return d
}

// Browse asks for a folder; "" if the user cancels.
func (b *Bridge) Browse(title string) (string, error) {
	return b.u.wails.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).SetTitle(title).PromptForSingleSelection()
}

// Setup runs the wizard. Setup-level failures are errors; per-flavor results are lines.
func (b *Bridge) Setup(a setup.Answers) ([]string, error) {
	res, err := b.u.app.Setup(a)
	switch {
	case errors.Is(err, syncdir.ErrAlreadyInitialized):
		return nil, errors.New(`this cloud folder already has wtfsync data: choose "Join my other PC"`)
	case errors.Is(err, syncdir.ErrNotInitialized):
		return nil, errors.New(`no wtfsync data in this cloud folder yet: set up your first PC, or choose "This is my first PC"`)
	case res.Settings.MachineUUID == "" && err != nil:
		return nil, err
	}
	return app.ResultLines(res), nil
}

func (b *Bridge) Conflicts() ConflictRequest { return b.u.prompt.request() }

// AnswerConflicts settles conflict question id: path → "remote" or "local".
func (b *Bridge) AnswerConflicts(id int, choices map[string]string, cancel bool) {
	b.u.prompt.reply(id, choices, cancel)
}

// Main is the main window's data.
func (b *Bridge) Main() app.MainPage { return b.u.app.Main() }

func (b *Bridge) PushAll() error               { return b.u.app.Push() }
func (b *Bridge) PullAll() error               { return b.u.app.Pull() }
func (b *Bridge) PushFlavor(name string) error { return b.u.app.PushFlavor(name) }
func (b *Bridge) PullFlavor(name string) error { return b.u.app.PullFlavor(name) }

// RestorePreview lists the files restoring backup id of flavor would rewrite or remove.
func (b *Bridge) RestorePreview(flavor, id string) ([]string, error) {
	r, err := b.u.app.RestorePreview(flavor, id)
	return list(r.Paths), err
}

// Restore puts backup id of flavor back and returns the id of the backup of what it replaced.
func (b *Bridge) Restore(flavor, id string) (string, error) {
	r, err := b.u.app.Restore(flavor, id)
	return r.Undo, err
}

func (b *Bridge) SetAppearance(v string) error              { return b.u.app.SetAppearance(v) }
func (b *Bridge) CheckUpdate() error                        { return b.u.app.CheckUpdate(context.Background()) }
func (b *Bridge) InstallUpdate()                            { go b.u.installUpdate() }
func (b *Bridge) OpenLogFolder()                            { b.u.openLogFolder() }
func (b *Bridge) CloudFolder(folder string) app.CloudFolder { return b.u.app.CloudFolder(folder) }

// FlavorNames lists a WoW folder's flavors, for the Settings switches when the folder changes.
func (b *Bridge) FlavorNames(wowDir string) []string { return b.u.app.FlavorNames(wowDir) }

// OpenURL opens an addon's download page; only https links are followed.
func (b *Bridge) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("not opening %q", url)
	}
	return b.u.wails.Browser.OpenURL(url)
}

type FlavorToggle struct {
	Name string
	On   bool
}

type SettingsForm struct {
	Name         string
	WoWDir       string
	SyncFolder   string
	AllFlavors   bool
	Flavors      []FlavorToggle // every flavor in the WoW folder; used when AllFlavors is off
	StartAtLogin bool
}

func (b *Bridge) Settings() (SettingsForm, error) {
	s, ok, err := b.u.app.Settings()
	if err != nil || !ok {
		return SettingsForm{}, cmp.Or(err, errors.New("this PC isn't set up yet"))
	}
	f := SettingsForm{Name: s.MachineName, WoWDir: s.WoWDir, SyncFolder: s.SyncFolder, AllFlavors: s.All(),
		StartAtLogin: b.u.app.StartsAtLogin(), Flavors: []FlavorToggle{}}
	flavors, _ := setup.Flavors(s.WoWDir)
	for _, fl := range flavors {
		f.Flavors = append(f.Flavors, FlavorToggle{Name: fl.Name, On: s.Enabled(fl.Name)})
	}
	slices.SortFunc(f.Flavors, func(a, b FlavorToggle) int { return strings.Compare(a.Name, b.Name) })
	return f, nil
}

func (b *Bridge) SaveSettings(f SettingsForm) ([]string, error) {
	in := app.SettingsInput{Name: f.Name, WoWDir: f.WoWDir, SyncFolder: f.SyncFolder, AllFlavors: f.AllFlavors,
		StartAtLogin: f.StartAtLogin, Flavors: map[string]bool{}}
	for _, t := range f.Flavors {
		in.Flavors[t.Name] = t.On
	}
	res, err := b.u.app.SaveSettings(in)
	if res.Settings.MachineUUID == "" && err != nil {
		return nil, err
	}
	lines := app.ResultLines(res)
	if err != nil {
		lines = append(lines, err.Error())
	}
	return list(lines), nil
}
