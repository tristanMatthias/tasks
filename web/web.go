// Package web embeds the static UI assets (moved here from beads_ui/static) so
// the server ships as a single self-contained binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var embedded embed.FS

// Static returns the embedded static/ directory as a filesystem rooted so that
// "index.html", "app.js", "styles.css" are at the top level.
func Static() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err) // build-time guarantee: static/ exists
	}
	return sub
}
