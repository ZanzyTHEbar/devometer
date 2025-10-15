package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// GetDistFS returns the embedded frontend distribution filesystem
func GetDistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
