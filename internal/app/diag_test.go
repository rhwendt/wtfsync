package app

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/setup"
)

// logBuf is a log the app can write from any goroutine.
type logBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *logBuf) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

func logged(a *App) *logBuf {
	b := &logBuf{}
	a.d.Log = log.New(b, "", 0)
	return b
}

func TestActionsAreLoggedWithHowLongTheyTook(t *testing.T) {
	_, _, _, nyApp := linked(t)
	out := logged(nyApp)
	if err := nyApp.Sync(); err != nil {
		t.Fatal(err)
	}
	contains(t, out.String(), "syncing: started")
	contains(t, out.String(), "syncing: done in ")
}

func TestFailedSetupIsLogged(t *testing.T) {
	ny, _ := newPair(t)
	a := ny.app()
	out := logged(a)
	if _, err := a.Setup(setup.Answers{Name: "NY"}); err == nil {
		t.Fatal("setup with no folders succeeded")
	}
	contains(t, out.String(), "setting up: failed: ")
}

func TestStalledActionLogsWhatEveryGoroutineIsDoing(t *testing.T) {
	_, _, _, nyApp := linked(t)
	out := logged(nyApp)
	nyApp.d.StallAfter = 20 * time.Millisecond
	nyApp.d.Stacks = func() []byte { return []byte("goroutine 1 [running]: here") }
	nyApp.begin("pulling") // e.g. a cloud folder that never answers
	time.Sleep(150 * time.Millisecond)
	nyApp.end()
	contains(t, out.String(), "pulling: still running after 20ms")
	contains(t, out.String(), "goroutine 1 [running]: here")
}

func TestQuickActionDumpsNothing(t *testing.T) {
	_, _, _, nyApp := linked(t)
	out := logged(nyApp)
	nyApp.d.StallAfter = time.Hour
	nyApp.d.Stacks = func() []byte { t.Error("stacks dumped for a quick action"); return nil }
	if err := nyApp.Sync(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if strings.Contains(out.String(), "still running") {
		t.Fatalf("log: %s", out)
	}
}

func TestStalledTickIsLoggedButQuickTicksAreNot(t *testing.T) {
	_, _, _, nyApp := linked(t)
	out := logged(nyApp)
	nyApp.d.StallAfter = 20 * time.Millisecond
	nyApp.d.Stacks = func() []byte { return []byte("stacks") }
	stop := nyApp.watch("tick", false)
	stop()
	if out.String() != "" {
		t.Fatalf("quick tick logged: %q", out)
	}
	stop = nyApp.watch("tick", false)
	time.Sleep(150 * time.Millisecond)
	stop()
	contains(t, out.String(), "tick: still running after 20ms")
	contains(t, out.String(), "tick: done in ")
}

func TestMenuAlwaysOffersTheLogFolder(t *testing.T) {
	for _, v := range []View{{}, {SetUp: true}, {SetUp: true, Busy: "pulling"}} {
		var found bool
		for _, it := range Menu(v) {
			if it.ID == "logs" {
				found = it.Enabled && it.Label == "Open log folder"
			}
		}
		if !found {
			t.Errorf("no enabled Open log folder in %+v", Menu(v))
		}
	}
}
