package ui

import (
	"sync"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// ConflictRequest is what the conflict dialog shows.
type ConflictRequest struct {
	Other     string   // the other PC's display name
	FirstLink bool     // this PC has never synced the flavor: offer the one-question choice
	Files     []string // conflicting paths, relative to WTF/
}

// prompter is the engine's BatchPrompter: it opens the conflict dialog and waits for the answer.
// The dialog reads the request with request and answers with reply; closing the window cancels.
type prompter struct {
	open func(window string)

	mu      sync.Mutex
	pending *ConflictRequest
	answer  chan map[string]engine.Choice // nil map = cancelled
}

func (p *prompter) Resolve(conflicts []string, other string, firstLink bool) (map[string]engine.Choice, error) {
	ch := make(chan map[string]engine.Choice, 1)
	p.mu.Lock()
	p.pending = &ConflictRequest{Other: other, FirstLink: firstLink, Files: conflicts}
	p.answer = ch
	p.mu.Unlock()
	p.open("conflicts")
	choices := <-ch
	if choices == nil {
		return nil, engine.ErrAborted
	}
	return choices, nil
}

// request is the dialog's current question; Files is empty when nothing is waiting.
func (p *prompter) request() ConflictRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == nil {
		return ConflictRequest{}
	}
	return *p.pending
}

// reply answers the waiting question once: choices maps path → "remote" or "local"; cancel (or
// closing the window) answers nil. Later replies, and replies with nothing waiting, are ignored.
func (p *prompter) reply(choices map[string]string, cancel bool) {
	p.mu.Lock()
	ch := p.answer
	p.pending, p.answer = nil, nil
	p.mu.Unlock()
	if ch == nil {
		return
	}
	if cancel {
		ch <- nil
		return
	}
	out := map[string]engine.Choice{}
	for path, c := range choices {
		switch c {
		case "remote":
			out[path] = engine.ChooseRemote
		case "local":
			out[path] = engine.ChooseLocal
		}
	}
	ch <- out
}

// The engine only calls these when a Prompter isn't a BatchPrompter; they route through the
// same dialog so behaviour can't differ.
func (p *prompter) FirstLink(int, string) (engine.FirstLinkChoice, error) {
	return engine.PerFile, nil
}

func (p *prompter) Conflict(path, other string) (engine.Choice, error) {
	m, err := p.Resolve([]string{path}, other, false)
	if err != nil {
		return 0, err
	}
	c, ok := m[path]
	if !ok {
		return 0, engine.ErrAborted
	}
	return c, nil
}
