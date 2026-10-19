package wow

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestProcessCheckerMatchesByFlavorFolder(t *testing.T) {
	root := filepath.Join("C:", "Games", "World of Warcraft")
	retail := Flavor{Name: "retail", Dir: filepath.Join(root, "_retail_")}
	classic := Flavor{Name: "classic", Dir: filepath.Join(root, "_classic_")}
	era := Flavor{Name: "classic_era", Dir: filepath.Join(root, "_classic_era_")}

	tests := []struct {
		name    string
		running []string
		flavor  Flavor
		want    bool
	}{
		{"retail running", []string{filepath.Join(root, "_retail_", "Wow.exe")}, retail, true},
		{"case-insensitive", []string{filepath.Join(root, "_RETAIL_", "WOW.EXE")}, retail, true},
		{"other flavor running", []string{filepath.Join(root, "_retail_", "Wow.exe")}, classic, false},
		{"era does not block classic", []string{filepath.Join(root, "_classic_era_", "WowClassic.exe")}, classic, false},
		{"era running", []string{filepath.Join(root, "_classic_era_", "WowClassic.exe")}, era, true},
		{"non-game exe in folder", []string{filepath.Join(root, "_retail_", "Utils", "Helper.exe")}, retail, false},
		{"nothing running", nil, retail, false},
		{"game with unknown path blocks every flavor", []string{"WowClassic.exe"}, retail, true},
		{"other exe with unknown path", []string{"Battle.net.exe"}, retail, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ProcessChecker{List: func() ([]string, error) { return tt.running, nil }}
			got, err := c.Running(tt.flavor)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Running = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessCheckerListError(t *testing.T) {
	boom := errors.New("boom")
	c := ProcessChecker{List: func() ([]string, error) { return nil, boom }}
	if _, err := c.Running(Flavor{Dir: "x"}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
