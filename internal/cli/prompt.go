package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// prompter asks questions on the terminal. End of input counts as aborting.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

func (p prompter) ask(q string) (string, error) {
	fmt.Fprint(p.out, q)
	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(p.out)
		return "", engine.ErrAborted
	}
	return strings.TrimSpace(line), nil
}

// line asks for a value, using def when the answer is empty. One pair of surrounding quotes is
// dropped, since Explorer's "Copy as path" adds them and cmd would have removed them from a flag.
func (p prompter) line(q, def string) (string, error) {
	if def != "" {
		q = fmt.Sprintf("%s [%s]: ", q, def)
	} else {
		q += ": "
	}
	a, err := p.ask(q)
	if err != nil {
		return "", err
	}
	a = unquote(a)
	if a == "" {
		a = def
	}
	if a == "" {
		return "", errors.New("no answer given")
	}
	return a, nil
}

func (p prompter) FirstLink(n int, other string) (engine.FirstLinkChoice, error) {
	q := fmt.Sprintf("This PC and %s both have settings for %d file(s).\n"+
		"  1) use %s's settings for all of them (recommended)\n"+
		"  2) keep this PC's\n"+
		"  3) ask for each file\n"+
		"  a) abort\n"+
		"Choice [1]: ", other, n, other)
	for {
		a, err := p.ask(q)
		if err != nil {
			return 0, err
		}
		switch strings.ToLower(a) {
		case "", "1":
			return engine.AllRemote, nil
		case "2":
			return engine.AllLocal, nil
		case "3":
			return engine.PerFile, nil
		case "a":
			return 0, engine.ErrAborted
		}
		fmt.Fprintln(p.out, "Please answer 1, 2, 3 or a.")
	}
}

func (p prompter) Conflict(path, other string) (engine.Choice, error) {
	q := fmt.Sprintf("%s changed on %s and on this PC.\n"+
		"  r) keep %s's version\n"+
		"  l) keep this PC's version\n"+
		"  a) abort\n"+
		"Choice: ", path, other, other)
	for {
		a, err := p.ask(q)
		if err != nil {
			return 0, err
		}
		switch strings.ToLower(a) {
		case "r":
			return engine.ChooseRemote, nil
		case "l":
			return engine.ChooseLocal, nil
		case "a":
			return 0, engine.ErrAborted
		}
		fmt.Fprintln(p.out, "Please answer r, l or a.")
	}
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}
