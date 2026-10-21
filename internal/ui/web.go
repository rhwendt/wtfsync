package ui

import (
	"embed"
	"io/fs"
)

//go:embed web
var webFiles embed.FS

// assets are the pages the windows load. Wails refuses to serve any of them unless the folder
// has an index.html, even though no window opens it.
func assets() (fs.FS, error) {
	return fs.Sub(webFiles, "web")
}
