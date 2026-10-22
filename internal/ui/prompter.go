package ui

import (
	"sync"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// ConflictRequest is what the conflict dialog shows.
type ConflictRequest struct {
	ID        int      // which question this is; replies and window closes name it
	Flavor    string   // the flavor being synced ("retail", "classic", …)
	Other     string   // the other PC's display name
	FirstLink bool     // this PC has never synced the flavor: offer the one-question choice
	Files     []string // conflicting paths, relative to WTF/
}

// prompter is the engine's BatchPrompter: it opens the conflict dialog and waits for the answer.
// The dialog reads the request with request and answers with reply; closing the window cancels.
// Every question gets a new ID so a reply or close meant for an earlier dialog can't answer it.
type prompter struct {
	open func(window string)

	mu      sync.Mutex
	last    int  // ID of the latest question
	closed  bool // shutdown was called: every question is cancelled
	pending *ConflictRequest
	answer  chan map[string]engine.Choice // nil map = cancelled
}

func (p *prompter) Resolve(flavor string, conflicts []string, other string, firstLink bool) (map[string]engine.Choice, error) {
	ch := make(chan map[string]engine.Choice, 1)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, engine.ErrAborted
	}
	p.last++
	p.pending = &ConflictRequest{ID: p.last, Flavor: flavor, Other: other, FirstLink: firstLink, Files: conflicts}
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

// reply answers question id once: choices maps path → "remote" or "local"; cancel (or closing
// the window) answers nil. Replies to any other question, or a second reply, are ignored.
func (p *prompter) reply(id int, choices map[string]string, cancel bool) {
	p.mu.Lock()
	if p.pending == nil || p.pending.ID != id {
		p.mu.Unlock()
		return
	}
	ch := p.answer
	p.pending, p.answer = nil, nil
	p.mu.Unlock()
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

// shutdown cancels the open question, if any, and every later one (the tray is quitting).
func (p *prompter) shutdown() {
	p.mu.Lock()
	p.closed = true
	id := p.last
	p.mu.Unlock()
	p.reply(id, nil, true)
}

// The engine only calls these when a Prompter isn't a BatchPrompter; they route through the
// same dialog so behaviour can't differ.
func (p *prompter) FirstLink(int, string) (engine.FirstLinkChoice, error) {
	return engine.PerFile, nil
}

func (p *prompter) Conflict(path, other string) (engine.Choice, error) {
	m, err := p.Resolve("", []string{path}, other, false) // never reached: the engine uses Resolve
	if err != nil {
		return 0, err
	}
	c, ok := m[path]
	if !ok {
		return 0, engine.ErrAborted
	}
	return c, nil
}
