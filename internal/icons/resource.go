package icons

import (
	"bytes"
	"image/png"
	"io"

	"github.com/tc-hib/winres"
)

// The IDs Wails loads the exe's icon by: 3 for each window, IDI_APPLICATION for its window class.
// Explorer and the Start menu shortcut show the lowest.
var appIconIDs = []winres.ID{3, 32512}

// Resource writes an amd64 COFF object holding AppOnDark as the exe's icon. The exe can't switch
// with the Windows theme; the windows do (see ui.setIcons). Saved as
// rsrc_windows_amd64.syso beside a main package, go build links it in (see .github/scripts/build.sh).
func Resource(w io.Writer) error {
	rs, err := resources()
	if err != nil {
		return err
	}
	return rs.WriteObject(w, winres.ArchAMD64)
}

func resources() (*winres.ResourceSet, error) {
	img, err := png.Decode(bytes.NewReader(AppOnDark))
	if err != nil {
		return nil, err
	}
	icon, err := winres.NewIconFromResizedImage(img, []int{256, 64, 48, 32, 24, 16})
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	for _, id := range appIconIDs {
		if err := rs.SetIcon(id, icon); err != nil {
			return nil, err
		}
	}
	return rs, nil
}
