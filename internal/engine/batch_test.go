package engine

import (
	"errors"
	"reflect"
	"testing"
)

// batch answers every conflict at once, like the tray's conflict dialog.
type batch struct {
	scripted // a BatchPrompter is still a Prompter
	choices  map[string]Choice
	err      error
	asked    []string
	first    bool
}

func (b *batch) Resolve(conflicts []string, _ string, firstLink bool) (map[string]Choice, error) {
	b.asked, b.first = conflicts, firstLink
	return b.choices, b.err
}

// conflicted is NY synced at s1, then both NJ (s2) and this PC changed A and B.
func conflicted(t *testing.T, p Prompter) *env {
	v := newEnv(t)
	v.eng.Prompt = p
	s1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	v.synced(s1, map[string]string{pathA: "a1", pathB: "b1"})
	v.writeLocal(pathA, "a-mine")
	v.writeLocal(pathB, "b-mine")
	v.remote(2, map[string]string{pathA: "a-nj", pathB: "b-nj"})
	return v
}

func TestBatchPrompterSettlesAllConflictsInOneCall(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseRemote, pathB: ChooseLocal}}
	v := conflicted(t, b)
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if want := []string{pathB, pathA}; !reflect.DeepEqual(b.asked, want) || b.first {
		t.Fatalf("asked %v (firstLink %v), want %v", b.asked, b.first, want)
	}
	v.want(pathA, "a-nj")
	v.want(pathB, "b-mine")
}

func TestBatchPrompterMissingChoiceWritesNothing(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseRemote}} // nothing for B
	v := conflicted(t, b)
	before := v.state().Base
	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	v.want(pathA, "a-mine")
	if v.state().Base != before {
		t.Fatal("base moved")
	}
}

func TestBatchPrompterCancelWritesNothing(t *testing.T) {
	b := &batch{err: ErrAborted}
	v := conflicted(t, b)
	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	v.want(pathA, "a-mine")
	v.want(pathB, "b-mine")
}

func TestBatchPrompterToldAboutFirstLink(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseLocal}}
	v := newEnv(t)
	v.eng.Prompt = b
	v.writeLocal(pathA, "a-mine")
	v.remote(1, map[string]string{pathA: "a-nj"})
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if !b.first {
		t.Fatal("firstLink = false on a PC that never synced")
	}
	v.want(pathA, "a-mine")
}
