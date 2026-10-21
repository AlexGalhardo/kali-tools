// Package frontend exposes the shared built UI to desktop and HTTP delivery.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the build rooted at dist/. The constant path is always valid.
func Dist() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}
