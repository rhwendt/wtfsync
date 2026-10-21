package ui

import (
	"io/fs"
	"testing"
)

func TestAssetsHaveTheIndexWailsNeeds(t *testing.T) {
	web, err := assets()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(web, "index.html"); err != nil {
		t.Fatalf("no index.html: %v", err)
	}
}
