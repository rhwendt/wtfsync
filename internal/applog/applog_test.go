package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatesPastCapKeepingOnePreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "tray.log")
	l, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"aaaaaa\n", "bbbbbb\n", "cccccc\n"} {
		if _, err := l.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()
	cur, _ := os.ReadFile(path)
	prev, _ := os.ReadFile(path + ".1")
	if string(cur) != "cccccc\n" || string(prev) != "bbbbbb\n" {
		t.Fatalf("current %q, previous %q", cur, prev)
	}
}

func TestReopenAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.log")
	for _, s := range []string{"one\n", "two\n"} {
		l, err := Open(path, DefaultMax)
		if err != nil {
			t.Fatal(err)
		}
		l.Write([]byte(s))
		l.Close()
	}
	if b, _ := os.ReadFile(path); !strings.HasPrefix(string(b), "one\ntwo\n") {
		t.Fatalf("log %q", b)
	}
}
