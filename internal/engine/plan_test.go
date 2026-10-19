package engine

import (
	"reflect"
	"testing"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name                string
		base, local, remote string
		want                Action
	}{
		{"unchanged", "a", "a", "a", NoOp},
		{"remote edited", "a", "a", "b", TakeRemote},
		{"remote deleted", "a", "a", "", TakeRemote},
		{"local edited", "a", "b", "a", KeepLocal},
		{"local deleted", "a", "", "a", KeepLocal},
		{"both edited differently", "a", "b", "c", Conflict},
		{"local edited, remote deleted", "a", "b", "", Conflict},
		{"local deleted, remote edited", "a", "", "b", Conflict},
		{"both made the same edit", "a", "b", "b", NoOp},
		{"both deleted", "a", "", "", NoOp},
		{"new on remote", "", "", "b", TakeRemote},
		{"new here", "", "b", "", KeepLocal},
		{"new on both, different (first link)", "", "b", "c", Conflict},
		{"new on both, same", "", "b", "b", NoOp},
	}
	for _, tt := range tests {
		if got := Decide(tt.base, tt.local, tt.remote); got != tt.want {
			t.Errorf("%s: Decide(%q,%q,%q) = %v, want %v", tt.name, tt.base, tt.local, tt.remote, got, tt.want)
		}
	}
}

func TestPlan(t *testing.T) {
	base := map[string]string{"a": "1", "b": "1", "c": "1"}
	local := map[string]string{"a": "1", "b": "2", "c": "1", "d": "1"}
	remote := map[string]string{"a": "2", "b": "3", "e": "1"}
	want := []Item{
		{Path: "a", Action: TakeRemote},
		{Path: "b", Action: Conflict},
		{Path: "c", Action: TakeRemote}, // deleted on remote
		{Path: "d", Action: KeepLocal},
		{Path: "e", Action: TakeRemote},
	}
	if got := Plan(base, local, remote); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestChanged(t *testing.T) {
	if n := changed(map[string]string{"a": "1", "b": "1"}, map[string]string{"a": "1", "b": "2", "c": "1"}); n != 2 {
		t.Fatalf("changed = %d, want 2", n)
	}
	if n := changed(nil, nil); n != 0 {
		t.Fatalf("changed(nil, nil) = %d", n)
	}
}

func TestActionString(t *testing.T) {
	if TakeRemote.String() != "take" || Conflict.String() != "conflict" {
		t.Fatal("bad Action strings")
	}
}
