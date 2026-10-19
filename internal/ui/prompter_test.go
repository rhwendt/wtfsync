package ui

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// dialog fakes the conflict window: when opened, it reads the request and replies.
func dialog(t *testing.T, reply func(p *prompter, req ConflictRequest)) *prompter {
	p := &prompter{}
	p.open = func(window string) {
		if window != "conflicts" {
			t.Errorf("opened %q", window)
		}
		go reply(p, p.request())
	}
	return p
}

func TestResolveReturnsTheDialogsChoices(t *testing.T) {
	var seen ConflictRequest
	p := dialog(t, func(p *prompter, req ConflictRequest) {
		seen = req
		p.reply(req.ID, map[string]string{"a.lua": "remote", "b.lua": "local", "c.lua": "bogus"}, false)
	})
	got, err := p.Resolve([]string{"a.lua", "b.lua"}, "NJ", true)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]engine.Choice{"a.lua": engine.ChooseRemote, "b.lua": engine.ChooseLocal}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if seen.Other != "NJ" || !seen.FirstLink || len(seen.Files) != 2 {
		t.Fatalf("request %+v", seen)
	}
	if p.request().Files != nil {
		t.Fatal("request still pending after the reply")
	}
}

func TestCancelOrCloseAborts(t *testing.T) {
	p := dialog(t, func(p *prompter, req ConflictRequest) { p.reply(req.ID, nil, true) })
	if _, err := p.Resolve([]string{"a.lua"}, "NJ", false); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
}

func TestSecondReplyIsIgnored(t *testing.T) {
	p := dialog(t, func(p *prompter, req ConflictRequest) {
		p.reply(req.ID, map[string]string{"a.lua": "local"}, false)
		p.reply(req.ID, nil, true) // e.g. the window closing right after Apply
	})
	got, err := p.Resolve([]string{"a.lua"}, "NJ", false)
	if err != nil || got["a.lua"] != engine.ChooseLocal {
		t.Fatalf("got %v, %v", got, err)
	}
	p.reply(0, nil, true) // nothing waiting: harmless
}

func TestPerFileConflictUsesTheDialog(t *testing.T) {
	p := dialog(t, func(p *prompter, req ConflictRequest) { p.reply(req.ID, map[string]string{"a.lua": "remote"}, false) })
	if c, err := p.Conflict("a.lua", "NJ"); err != nil || c != engine.ChooseRemote {
		t.Fatalf("got %v, %v", c, err)
	}
}

// Two flavors in conflict open two dialogs in a row. The first window's close event can arrive
// after the second question is already waiting: it must not cancel it.
func TestStaleCloseDoesNotCancelTheNextQuestion(t *testing.T) {
	var first ConflictRequest
	p := &prompter{}
	p.open = func(string) {
		req := p.request()
		if first.ID == 0 {
			first = req
			go p.reply(req.ID, map[string]string{"a.lua": "local"}, false)
			return
		}
		if req.ID == first.ID {
			t.Errorf("second question reused ID %d", req.ID)
		}
		go func() {
			p.reply(first.ID, nil, true) // the first window closing late
			p.reply(req.ID, map[string]string{"b.lua": "remote"}, false)
		}()
	}
	if got, err := p.Resolve([]string{"a.lua"}, "NJ", false); err != nil || got["a.lua"] != engine.ChooseLocal {
		t.Fatalf("first: %v, %v", got, err)
	}
	got, err := p.Resolve([]string{"b.lua"}, "NJ", false)
	if err != nil || got["b.lua"] != engine.ChooseRemote {
		t.Fatalf("second: got %v, %v; want b.lua kept from NJ", got, err)
	}
}

// Quitting the tray cancels the open question and any the sync would ask next.
func TestShutdownCancelsNowAndLater(t *testing.T) {
	p := &prompter{}
	opened := make(chan struct{}, 2)
	p.open = func(string) { opened <- struct{}{} }
	errs := make(chan error)
	go func() { _, err := p.Resolve([]string{"a.lua"}, "NJ", false); errs <- err }()
	<-opened
	p.shutdown()
	if err := <-errs; !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("open question: err = %v", err)
	}
	if _, err := p.Resolve([]string{"b.lua"}, "NJ", false); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("later question: err = %v", err)
	}
	if len(opened) != 0 {
		t.Fatal("opened a dialog after shutdown")
	}
}
