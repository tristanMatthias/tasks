package buildinfo

import "testing"

func TestBuildInfo(t *testing.T) {
	if Short() == "" {
		t.Error("Short empty")
	}
	if String() == "" {
		t.Error("String empty")
	}
	m := Map()
	for _, k := range []string{"version", "commit", "date", "go", "os", "arch"} {
		if _, ok := m[k]; !ok {
			t.Errorf("Map missing %q", k)
		}
	}
	// Short with a long commit is truncated to 12 chars + version.
	old := Commit
	Commit = "0123456789abcdef"
	if s := Short(); len(s) == 0 {
		t.Error("Short with commit empty")
	}
	Commit = ""
	if Short() != Version {
		t.Error("Short without commit should be Version")
	}
	Commit = old
}
