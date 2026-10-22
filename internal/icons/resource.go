package icons

import (
	"bytes"
	"image/png"
	"io"

	"github.com/tc-hib/winres"
)

// appIconID is IDI_APPLICATION, the icon Wails loads from the exe for its window class. As the
// exe's only icon it's also the one Explorer, the Start menu shortcut and the taskbar show.
const appIconID = 32512

// Resource writes an amd64 COFF object holding App as the exe's icon. Saved as
// rsrc_windows_amd64.syso beside a main package, go build links it in (see .github/scripts/build.sh).
func Resource(w io.Writer) error {
	rs, err := resources()
	if err != nil {
		return err
	}
	return rs.WriteObject(w, winres.ArchAMD64)
}

func resources() (*winres.ResourceSet, error) {
	img, err := png.Decode(bytes.NewReader(App))
	if err != nil {
		return nil, err
	}
	icon, err := winres.NewIconFromResizedImage(img, []int{256, 64, 48, 32, 24, 16})
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(appIconID), icon); err != nil {
		return nil, err
	}
	return rs, nil
}
