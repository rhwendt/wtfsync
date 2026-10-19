package engine

import (
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// OtherAddons lists the addons another PC has for a flavor that this PC doesn't.
type OtherAddons struct {
	Machine string // the other PC's display name
	Addons  []collect.MissingAddon
}

// missingAddons compares this PC's installed addons with every other PC's last-pushed list. It's
// advice, not sync: a list that can't be read is skipped rather than failing the command.
func (e *Engine) missingAddons(f wow.Flavor, d *syncdir.Dir) []OtherAddons {
	lists, err := d.AddonLists(f.Name)
	if err != nil || len(lists) == 0 {
		return nil
	}
	installed, err := collect.Addons(f.AddOnsDir())
	if err != nil {
		return nil
	}
	var out []OtherAddons
	for _, l := range lists {
		if l.UUID == e.Settings.MachineUUID {
			continue
		}
		if m := collect.Missing(installed, l.Addons); m != nil {
			out = append(out, OtherAddons{Machine: l.Name, Addons: m})
		}
	}
	return out
}

// recordAddons publishes this PC's installed addons for f so other PCs can tell what they lack.
func (e *Engine) recordAddons(f wow.Flavor, d *syncdir.Dir, addons []collect.Addon) error {
	return d.WriteAddons(f.Name, syncdir.AddonList{
		UUID: e.Settings.MachineUUID, Name: e.Settings.MachineName, Updated: e.Now(), Addons: addons,
	})
}
