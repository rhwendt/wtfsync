package lock

import (
	"errors"
	"testing"
)

func TestSecondAcquireFailsUntilRelease(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir); !errors.Is(err, ErrHeld) {
		t.Fatalf("second acquire: err = %v, want ErrHeld", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := Acquire(dir)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again.Release()
}

func TestAcquireCreatesDataFolder(t *testing.T) {
	l, err := Acquire(t.TempDir() + "/not/yet")
	if err != nil {
		t.Fatal(err)
	}
	l.Release()
}

func TestReleaseTwiceIsHarmless(t *testing.T) {
	l, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("second release: %v", err)
	}
}
