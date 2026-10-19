package ui

import (
	"encoding/json"
	"testing"
)

// The pages loop over lists; JSON null (a nil slice) would throw and leave the window blank.
func TestListIsNeverNull(t *testing.T) {
	if b, _ := json.Marshal(list[string](nil)); string(b) != "[]" {
		t.Fatalf("nil list = %s, want []", b)
	}
	if got := list([]string{"a"}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v", got)
	}
}
