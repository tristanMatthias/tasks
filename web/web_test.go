package web

import (
	"io/fs"
	"testing"
)

func TestStaticEmbed(t *testing.T) {
	f := Static()
	for _, name := range []string{"index.html", "app.js", "styles.css"} {
		data, err := fs.ReadFile(f, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}
