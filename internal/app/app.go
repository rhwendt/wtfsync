package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rhwendt/wtfsync/internal/autostart"
	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/update"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// ErrBusy means an action is already running in this tray.
var ErrBusy = errors.New("wtfsync is busy")

// Notice is a notification (Windows toast).
type Notice struct {
	ID     string // one per kind of event and flavor, so a newer one replaces an older one
	Title  string
	Body   string
	Action string // what clicking it does: "pull", "addons", or "" (nothing)
}

// Updates is where new releases come from (update.Checker in the real app).
type Updates interface {
	Latest(ctx context.Context) (update.Release, error)
	Download(ctx context.Context, r update.Release, dir string) (string, error)
}

// Deps are the outside world: cmd/wtfsync-tray passes real ones, tests pass fakes.
type Deps struct {
	Store     local.Store
	Procs     wow.Processes
	Hydrator  syncdir.Hydrator
	Prompt    engine.Prompter // the conflict dialog
	Now       func() time.Time
	Updates   Updates
	Autostart autostart.Setting
	Version   string
	Notify    func(Notice) // may be nil
	Changed   func()       // called after the View changes; may be nil
	Log       *log.Logger  // may be nil

	// StallAfter is how long an action or tick may run before the log gets what every goroutine
	// is doing (0 means a minute); Stacks is where that comes from (nil means this process).
	StallAfter time.Duration
	Stacks     func() []byte
}

type App struct {
	d Deps

	act sync.Mutex // held by the running action or tick

	mu       sync.Mutex
	busy     string
	watching func() // ends the running action's watch
	closing  bool   // Close was called: no new actions or ticks
	flavors  map[string]*FlavorView
	update   *update.Release
	notified map[string]string // flavor → newest snapshot already announced
	missing  int               // missing addons already announced
}

func New(d Deps) *App {
	if d.Notify == nil {
		d.Notify = func(Notice) {}
	}
	if d.Changed == nil {
		d.Changed = func() {}
	}
	if d.Log == nil {
		d.Log = log.New(io.Discard, "", 0)
	}
	if d.StallAfter == 0 {
		d.StallAfter = time.Minute
	}
	if d.Stacks == nil {
		d.Stacks = stacks
	}
	return &App{d: d, flavors: map[string]*FlavorView{}, notified: map[string]string{}}
}

// View is a snapshot of the tray's state.
func (a *App) View() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.d.Store.LoadSettings()
	v := View{SetUp: err == nil, Busy: a.busy}
	for _, name := range slices.Sorted(maps.Keys(a.flavors)) {
		v.Flavors = append(v.Flavors, *a.flavors[name])
	}
	if a.update != nil {
		v.Update = a.update.Version
	}
	summarize(&v)
	return v
}

// Settings returns this PC's settings; ok is false when it isn't set up yet.
func (a *App) Settings() (s local.Settings, ok bool, err error) {
	s, err = a.d.Store.LoadSettings()
	if errors.Is(err, local.ErrNoSettings) {
		return s, false, nil
	}
	return s, err == nil, err
}

// enabled loads settings, the engine for them, and the enabled flavors.
func (a *App) enabled() (*engine.Engine, []wow.Flavor, error) {
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return nil, nil, err
	}
	all, err := wow.Flavors(s.WoWDir)
	if err != nil {
		return nil, nil, err
	}
	var on []wow.Flavor
	for _, f := range all {
		if s.Enabled(f.Name) {
			on = append(on, f)
		}
	}
	eng := &engine.Engine{Store: a.d.Store, Settings: s, Procs: a.d.Procs, Hydrator: a.d.Hydrator, Prompt: a.d.Prompt, Now: a.d.Now}
	return eng, on, nil
}

// Refresh reloads every enabled flavor's status. Errors from the last action stay until the
// next action on that flavor.
func (a *App) Refresh() {
	eng, flavors, err := a.enabled()
	if err != nil {
		a.mu.Lock()
		a.flavors = map[string]*FlavorView{}
		a.mu.Unlock()
		a.d.Changed()
		return
	}
	fresh := map[string]*FlavorView{}
	for _, f := range flavors {
		st, err := eng.Status(f)
		if err != nil {
			a.d.Log.Printf("status %s: %v", f.Name, err)
		}
		fresh[f.Name] = &FlavorView{Name: f.Name, Status: st}
	}
	a.mu.Lock()
	for name, fv := range fresh {
		if old, ok := a.flavors[name]; ok {
			fv.Err = old.Err
		}
	}
	a.flavors = fresh
	a.mu.Unlock()
	a.d.Changed()
}

// Tick runs every 60 s: retry pushes that couldn't reach the sync folder, refresh, and announce
// newer settings from other PCs (once per snapshot).
func (a *App) Tick() {
	a.mu.Lock()
	closing := a.closing
	a.mu.Unlock()
	if closing || !a.act.TryLock() {
		return // an action is running; it refreshes when it ends
	}
	stop := a.watch("tick", false)
	defer stop()
	if l, err := lock.Acquire(a.d.Store.Dir); err == nil {
		if eng, flavors, err := a.enabled(); err == nil {
			a.retryPending(eng, flavors)
		}
		l.Release()
	}
	a.act.Unlock()
	a.Refresh()
	for _, f := range a.View().Flavors {
		a.mu.Lock()
		announce := f.newer() && a.notified[f.Name] != f.Status.Newest
		if announce {
			a.notified[f.Name] = f.Status.Newest
		}
		a.mu.Unlock()
		if announce {
			a.d.Notify(Notice{ID: "new-" + f.Name, Title: "New settings for " + f.Name,
				Body: f.Status.NewestBy + " pushed new settings.", Action: "pull"})
		}
	}
}

func (a *App) retryPending(eng *engine.Engine, flavors []wow.Flavor) {
	for _, f := range flavors {
		st, err := a.d.Store.LoadState(f.Name)
		if err != nil || !st.PendingPush {
			continue
		}
		if _, err := eng.Push(f, engine.Options{}); err != nil {
			a.d.Log.Printf("retry push %s: %v", f.Name, err)
		}
	}
}

// begin marks the tray busy with what, waiting for a tick in progress. It returns false if
// another action is already running.
func (a *App) begin(what string) bool {
	a.mu.Lock()
	if a.busy != "" || a.closing {
		a.mu.Unlock()
		return false
	}
	a.busy = what
	a.mu.Unlock()
	a.act.Lock()
	stop := a.watch(what, true)
	a.mu.Lock()
	a.watching = stop
	a.mu.Unlock()
	return true
}

// Close is called before the tray quits. It waits up to wait for a running action or tick to
// finish, so quitting doesn't stop a pull halfway through writing files, then keeps new ones from
// starting (actions get ErrBusy; ticks skip). It reports whether nothing was running by then.
func (a *App) Close(wait time.Duration) bool {
	a.mu.Lock()
	a.closing = true
	a.mu.Unlock()
	deadline := time.Now().Add(wait)
	for !a.act.TryLock() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true // act stays held: nothing runs after this
}

func (a *App) end() {
	a.mu.Lock()
	stop := a.watching
	a.watching = nil
	a.mu.Unlock()
	if stop != nil {
		stop()
	}
	a.act.Unlock()
	a.mu.Lock()
	a.busy = ""
	a.mu.Unlock()
}

// watch logs how long what takes. If it runs past StallAfter, it also logs every goroutine's stack
// once, so a tray that seems stuck leaves a record of where. Quiet watches log only when slow.
func (a *App) watch(what string, loud bool) (stop func()) {
	start := time.Now()
	if loud {
		a.d.Log.Printf("%s: started", what)
	}
	var slow atomic.Bool
	timer := time.AfterFunc(a.d.StallAfter, func() {
		slow.Store(true)
		a.d.Log.Printf("%s: still running after %v; goroutines:\n%s", what, a.d.StallAfter, a.d.Stacks())
	})
	return func() {
		timer.Stop()
		if loud || slow.Load() {
			a.d.Log.Printf("%s: done in %v", what, time.Since(start).Round(time.Millisecond))
		}
	}
}

// stacks is every goroutine's stack in this process.
func stacks() []byte {
	for n := 1 << 16; ; n *= 2 {
		buf := make([]byte, n)
		if l := runtime.Stack(buf, true); l < n {
			return buf[:l]
		}
	}
}

// run does one user action: busy state, the per-PC lock, then fn per enabled flavor (or only the
// flavor named by only). Per-flavor errors are kept on the flavor and raised as notifications.
func (a *App) run(what, only string, fn func(eng *engine.Engine, f wow.Flavor) error) (err error) {
	if !a.begin(what) {
		return ErrBusy
	}
	a.d.Changed()
	defer func() {
		if err != nil {
			a.d.Log.Printf("%s: failed: %v", what, err)
		}
		a.end()
		a.Refresh()
	}()
	l, err := lock.Acquire(a.d.Store.Dir)
	if err != nil {
		if errors.Is(err, lock.ErrHeld) {
			a.d.Notify(Notice{ID: "busy", Title: "wtfsync", Body: "A sync is already running."})
		}
		return err
	}
	defer l.Release()
	eng, flavors, err := a.enabled()
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range flavors {
		if only != "" && f.Name != only {
			continue
		}
		err := fn(eng, f)
		a.mu.Lock()
		fv, ok := a.flavors[f.Name]
		if !ok {
			fv = &FlavorView{Name: f.Name}
			a.flavors[f.Name] = fv
		}
		fv.Err = ""
		if err != nil {
			fv.Err = friendly(f.Name, err)
		}
		a.mu.Unlock()
		if err != nil {
			a.d.Notify(Notice{ID: "error-" + f.Name, Title: "wtfsync: " + f.Name, Body: friendly(f.Name, err)})
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
		}
	}
	if only != "" && !slices.ContainsFunc(flavors, func(f wow.Flavor) bool { return f.Name == only }) {
		return fmt.Errorf("no enabled flavor %q", only)
	}
	return errors.Join(errs...)
}

// Push pushes every enabled flavor.
func (a *App) Push() error { return a.push("") }

// PushFlavor pushes one enabled flavor.
func (a *App) PushFlavor(name string) error { return a.push(name) }

func (a *App) push(only string) error {
	return a.run(doing("pushing", only), only, func(eng *engine.Engine, f wow.Flavor) error {
		_, err := eng.Push(f, engine.Options{})
		return err
	})
}

// Pull pulls every enabled flavor (retrying a pending push first, as the CLI does) and then
// announces addons other PCs have that this one doesn't, when that list has grown.
func (a *App) Pull() error { return a.pull("") }

// PullFlavor pulls one enabled flavor.
func (a *App) PullFlavor(name string) error { return a.pull(name) }

func (a *App) pull(only string) error {
	err := a.run(doing("pulling", only), only, func(eng *engine.Engine, f wow.Flavor) error {
		a.retryPending(eng, []wow.Flavor{f})
		_, err := eng.Pull(f, engine.Options{})
		return err
	})
	v := a.View()
	a.mu.Lock()
	announce := v.Missing > a.missing
	a.missing = v.Missing
	a.mu.Unlock()
	if announce {
		a.d.Notify(Notice{ID: "addons", Title: "Missing addons",
			Body: fmt.Sprintf("Your other PCs have %d addon(s) this PC doesn't.", v.Missing), Action: "addons"})
	}
	return err
}

// doing is the busy text: "pulling", or "pulling classic" for one flavor.
func doing(what, only string) string {
	if only == "" {
		return what
	}
	return what + " " + only
}

// RestorePreview lists what restoring one of flavor's backup sets (id "" = the newest) would change.
func (a *App) RestorePreview(flavor, id string) (engine.RestoreReport, error) {
	eng, f, err := a.flavor(flavor)
	if err != nil {
		return engine.RestoreReport{}, err
	}
	return eng.Restore(f, id, engine.Options{DryRun: true})
}

// Restore puts one of flavor's backup sets back (id "" = the newest), backing up what it replaces.
func (a *App) Restore(flavor, id string) (engine.RestoreReport, error) {
	var rep engine.RestoreReport
	err := a.run("restoring "+flavor, flavor, func(eng *engine.Engine, f wow.Flavor) error {
		var err error
		rep, err = eng.Restore(f, id, engine.Options{})
		return err
	})
	return rep, err
}

func (a *App) flavor(name string) (*engine.Engine, wow.Flavor, error) {
	eng, flavors, err := a.enabled()
	if err != nil {
		return nil, wow.Flavor{}, err
	}
	for _, f := range flavors {
		if f.Name == name {
			return eng, f, nil
		}
	}
	return nil, wow.Flavor{}, fmt.Errorf("no enabled flavor %q", name)
}

// Setup runs the wizard's answers (first setup, or a rerun).
func (a *App) Setup(ans setup.Answers) (res setup.Result, err error) {
	if !a.begin("setting up") {
		return setup.Result{}, ErrBusy
	}
	a.d.Changed()
	defer func() {
		if err != nil {
			a.d.Log.Printf("setting up: failed: %v", err)
		}
		a.end()
		a.Refresh()
	}()
	l, err := lock.Acquire(a.d.Store.Dir)
	if err != nil {
		return setup.Result{}, err
	}
	defer l.Release()
	return setup.Run(setup.Env{Store: a.d.Store, Procs: a.d.Procs, Hydrator: a.d.Hydrator, Prompt: a.d.Prompt, Now: a.d.Now}, ans)
}

// SettingsInput is the settings page's form.
type SettingsInput struct {
	Name         string
	WoWDir       string
	SyncFolder   string
	AllFlavors   bool
	Flavors      map[string]bool // used when AllFlavors is off
	StartAtLogin bool
}

// SaveSettings applies the settings page. Changing folders follows the setup rerun rules; a new
// sync folder that already has wtfsync data is joined (link), otherwise this PC starts it (init).
func (a *App) SaveSettings(in SettingsInput) (setup.Result, error) {
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return setup.Result{}, err
	}
	s.AllFlavors = &in.AllFlavors
	for name, on := range in.Flavors {
		s.Flavors[name] = on
	}
	if err := a.d.Store.SaveSettings(s); err != nil {
		return setup.Result{}, err
	}
	_, openErr := syncdir.Open(in.SyncFolder, a.d.Hydrator)
	res, err := a.Setup(setup.Answers{WoWDir: in.WoWDir, SyncFolder: in.SyncFolder, Name: in.Name, First: openErr != nil})
	if err != nil {
		return res, err
	}
	if on, aerr := a.d.Autostart.Enabled(); aerr != nil || on != in.StartAtLogin {
		if err := a.d.Autostart.Set(in.StartAtLogin); err != nil {
			return res, fmt.Errorf("start at login: %w", err)
		}
	}
	return res, nil
}

// CheckUpdate looks for a newer release and offers it. Failures are logged and returned but never
// raised as notifications: being offline mustn't nag.
func (a *App) CheckUpdate(ctx context.Context) error {
	if a.d.Updates == nil {
		return nil
	}
	r, err := a.d.Updates.Latest(ctx)
	if err != nil {
		a.d.Log.Printf("check for updates: %v", err)
		return err
	}
	a.mu.Lock()
	if update.Newer(a.d.Version, r.Version) {
		a.update = &r
	} else {
		a.update = nil
	}
	a.mu.Unlock()
	a.d.Changed()
	return nil
}

// InstallUpdate downloads the offered release into dir, waits for any sync to finish, then calls
// run with the installer's path. The caller quits the tray afterwards; the installer restarts it.
func (a *App) InstallUpdate(ctx context.Context, dir string, run func(installer string) error) error {
	a.mu.Lock()
	r := a.update
	a.mu.Unlock()
	if r == nil {
		return errors.New("no update available")
	}
	if !a.begin("updating") {
		return ErrBusy
	}
	a.d.Changed()
	defer func() {
		a.end()
		a.d.Changed()
	}()
	path, err := a.d.Updates.Download(ctx, *r, dir)
	if err != nil {
		a.d.Notify(Notice{ID: "update", Title: "Update failed", Body: err.Error()})
		return err
	}
	l, err := waitLock(ctx, a.d.Store.Dir)
	if err != nil {
		return err
	}
	defer l.Release()
	return run(path)
}

// waitLock waits for a sync in another process (the CLI) to finish.
func waitLock(ctx context.Context, dir string) (*lock.Lock, error) {
	for {
		l, err := lock.Acquire(dir)
		if !errors.Is(err, lock.ErrHeld) {
			return l, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// HasSyncData reports whether folder already holds wtfsync data, so the wizard can suggest
// "Join my other PC" rather than "This is my first PC".
func (a *App) HasSyncData(folder string) bool {
	_, err := syncdir.Open(folder, a.d.Hydrator)
	return err == nil
}

// Version is the running build's version ("dev" for local builds).
func (a *App) Version() string { return a.d.Version }

// StartsAtLogin reports the start-at-login setting; false when it can't be read.
func (a *App) StartsAtLogin() bool {
	if a.d.Autostart == nil {
		return false
	}
	on, err := a.d.Autostart.Enabled()
	return err == nil && on
}

// SetAppearance saves this PC's appearance: "system", "light" or "dark".
func (a *App) SetAppearance(v string) error {
	switch v {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("unknown appearance %q", v)
	}
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return err
	}
	s.Appearance = v
	if err := a.d.Store.SaveSettings(s); err != nil {
		return err
	}
	a.d.Changed()
	return nil
}

// CloudFolder is what the wizard needs to know about a cloud folder.
type CloudFolder struct {
	HasData bool     // the folder has wtfsync data: this PC joins it unless it's a rerun of the first PC
	PCs     []string // the other PCs whose records could be read; may be empty even with data
}

// CloudFolder says whether folder has wtfsync data and names the PCs in it other than this one.
// Whether to offer "Join" comes from HasData, not PCs: a streaming drive may not have downloaded
// the other PC's record yet.
func (a *App) CloudFolder(folder string) CloudFolder {
	c := CloudFolder{PCs: []string{}}
	d, err := syncdir.Open(folder, a.d.Hydrator)
	if err != nil {
		return c
	}
	c.HasData = true
	me, _ := a.d.Store.LoadSettings()
	machines, _ := d.Machines()
	for _, m := range machines {
		if m.UUID != me.MachineUUID {
			c.PCs = append(c.PCs, m.Name)
		}
	}
	return c
}
