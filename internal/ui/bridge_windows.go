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
	d := WizardDefaults{Installs: setup.FindInstalls(o.InstallDirs), Clouds: syncdir.Candidates(o.Getenv, fsutil.IsDir)}
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

func (b *Bridge) HasSyncData(folder string) bool { return b.u.app.HasSyncData(folder) }

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

// AnswerConflicts settles the waiting conflicts: path → "remote" or "local".
func (b *Bridge) AnswerConflicts(choices map[string]string, cancel bool) {
	b.u.prompt.reply(choices, cancel)
}

type StatusPage struct {
	Rows    []app.StatusRow
	Version string
}

func (b *Bridge) Status() StatusPage {
	b.u.app.Refresh()
	return StatusPage{Rows: app.StatusRows(b.u.app.View()), Version: b.u.app.Version()}
}

func (b *Bridge) Missing() []app.AddonRow { return app.AddonRows(b.u.app.View()) }

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
	Flavors      []FlavorToggle
	StartAtLogin bool
}

func (b *Bridge) Settings() (SettingsForm, error) {
	s, ok, err := b.u.app.Settings()
	if err != nil || !ok {
		return SettingsForm{}, cmp.Or(err, errors.New("this PC isn't set up yet"))
	}
	f := SettingsForm{Name: s.MachineName, WoWDir: s.WoWDir, SyncFolder: s.SyncFolder, StartAtLogin: b.u.app.StartsAtLogin()}
	flavors, _ := setup.Flavors(s.WoWDir)
	for _, fl := range flavors {
		on, listed := s.Flavors[fl.Name]
		f.Flavors = append(f.Flavors, FlavorToggle{Name: fl.Name, On: on || !listed})
	}
	slices.SortFunc(f.Flavors, func(a, b FlavorToggle) int { return strings.Compare(a.Name, b.Name) })
	return f, nil
}

func (b *Bridge) SaveSettings(f SettingsForm) ([]string, error) {
	in := app.SettingsInput{Name: f.Name, WoWDir: f.WoWDir, SyncFolder: f.SyncFolder, StartAtLogin: f.StartAtLogin, Flavors: map[string]bool{}}
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
	return lines, nil
}

type RestoreInfo struct {
	Flavor string
	Paths  []string
	Err    string // e.g. "no backups for retail yet"
}

func (b *Bridge) RestoreInfo() []RestoreInfo {
	var out []RestoreInfo
	for _, f := range b.u.app.View().Flavors {
		r, err := b.u.app.RestorePreview(f.Name)
		info := RestoreInfo{Flavor: f.Name, Paths: r.Paths}
		if err != nil {
			info.Err = err.Error()
		}
		out = append(out, info)
	}
	return out
}

func (b *Bridge) Restore(flavor string) (string, error) {
	r, err := b.u.app.Restore(flavor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Restored %d file(s). The files it replaced were backed up (%s).", len(r.Paths), r.Undo), nil
}
