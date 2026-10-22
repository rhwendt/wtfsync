package local

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if _, err := s.LoadSettings(); !errors.Is(err, ErrNoSettings) {
		t.Fatalf("err = %v, want ErrNoSettings", err)
	}
	want := Settings{MachineUUID: NewUUID(), MachineName: "NJ", SyncFolder: `G:\My Drive`, WoWDir: `C:\WoW`, Flavors: map[string]bool{"retail": true, "ptr": false}}
	if err := s.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadSettings()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	got, err := s.LoadState("retail")
	if err != nil || got.Base != "" || got.Files != nil || got.PendingPush {
		t.Fatalf("missing state = %+v, %v; want zero", got, err)
	}
	want := FlavorState{Base: "000001_x.tar.gz", Files: map[string]string{"a": "sha256:1"}, PendingPush: true}
	if err := s.SaveState("retail", want); err != nil {
		t.Fatal(err)
	}
	if got, err := s.LoadState("retail"); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestNewUUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	a, b := NewUUID(), NewUUID()
	if !re.MatchString(a) || a == b {
		t.Fatalf("NewUUID = %s, %s", a, b)
	}
}

func TestAllAndEnabled(t *testing.T) {
	on, off := true, false
	for _, tt := range []struct {
		name    string
		s       Settings
		all     bool
		enabled map[string]bool // flavor → want
	}{
		{"all on", Settings{AllFlavors: &on, Flavors: map[string]bool{"retail": false}}, true,
			map[string]bool{"retail": true, "classic": true}},
		{"all off picks", Settings{AllFlavors: &off, Flavors: map[string]bool{"retail": true, "classic": false}}, false,
			map[string]bool{"retail": true, "classic": false, "classic_era": false}},
		{"old file, nothing off", Settings{Flavors: map[string]bool{"retail": true}}, true,
			map[string]bool{"retail": true, "classic": true}},
		{"old file, one off", Settings{Flavors: map[string]bool{"retail": true, "classic": false}}, false,
			map[string]bool{"retail": true, "classic": false, "classic_era": false}},
	} {
		if got := tt.s.All(); got != tt.all {
			t.Errorf("%s: All() = %v, want %v", tt.name, got, tt.all)
		}
		for f, want := range tt.enabled {
			if got := tt.s.Enabled(f); got != want {
				t.Errorf("%s: Enabled(%s) = %v, want %v", tt.name, f, got, want)
			}
		}
	}
}

func TestThemeDefaultsToSystem(t *testing.T) {
	for in, want := range map[string]string{"": "system", "system": "system", "light": "light", "dark": "dark", "purple": "system"} {
		if got := (Settings{Appearance: in}).Theme(); got != want {
			t.Errorf("Theme(%q) = %q, want %q", in, got, want)
		}
	}
}
