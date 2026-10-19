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
		p.reply(map[string]string{"a.lua": "remote", "b.lua": "local", "c.lua": "bogus"}, false)
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
	p := dialog(t, func(p *prompter, _ ConflictRequest) { p.reply(nil, true) })
	if _, err := p.Resolve([]string{"a.lua"}, "NJ", false); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
}

func TestSecondReplyIsIgnored(t *testing.T) {
	p := dialog(t, func(p *prompter, _ ConflictRequest) {
		p.reply(map[string]string{"a.lua": "local"}, false)
		p.reply(nil, true) // e.g. the window closing right after Apply
	})
	got, err := p.Resolve([]string{"a.lua"}, "NJ", false)
	if err != nil || got["a.lua"] != engine.ChooseLocal {
		t.Fatalf("got %v, %v", got, err)
	}
	p.reply(nil, true) // nothing waiting: harmless
}

func TestPerFileConflictUsesTheDialog(t *testing.T) {
	p := dialog(t, func(p *prompter, _ ConflictRequest) { p.reply(map[string]string{"a.lua": "remote"}, false) })
	if c, err := p.Conflict("a.lua", "NJ"); err != nil || c != engine.ChooseRemote {
		t.Fatalf("got %v, %v", c, err)
	}
}
