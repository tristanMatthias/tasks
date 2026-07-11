package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestStaticEmbed(t *testing.T) {
	f := Static()
	// index.html is the SPA entry.
	data, err := fs.ReadFile(f, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("index.html is empty")
	}
	// The Vite build emits hashed JS/CSS under assets/.
	entries, err := fs.ReadDir(f, "assets")
	if err != nil {
		t.Fatalf("read assets/: %v", err)
	}
	var hasJS, hasCSS bool
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e.Name(), ".js"):
			hasJS = true
		case strings.HasSuffix(e.Name(), ".css"):
			hasCSS = true
		}
	}
	if !hasJS || !hasCSS {
		t.Fatalf("expected built js+css in assets/, got %v", entries)
	}
}
