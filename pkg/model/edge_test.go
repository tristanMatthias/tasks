package model

import (
	"errors"
	"strings"
	"testing"
)

func TestNaturalOrdering(t *testing.T) {
	cases := []struct {
		a, b string
		less bool
	}{
		{"ps3t.2", "ps3t.11", true}, // numeric run compared as ints
		{"ps3t.11", "ps3t.2", false},
		{"proj-007", "proj-7", false}, // equal numerically (leading zeros)
		{"proj-7", "proj-007", false}, // equal -> not less
		{"a", "b", true},
		{"abc", "abcd", true}, // prefix shorter first
		{"a1", "a1", false},
		{"10", "9", false}, // 10 > 9
	}
	for _, c := range cases {
		if got := NaturalLess(c.a, c.b); got != c.less {
			t.Errorf("NaturalLess(%q,%q) = %v, want %v", c.a, c.b, got, c.less)
		}
	}
}

// failWriter errors after n successful bytes to exercise WriteJSONL's error path.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

func TestWriteJSONLError(t *testing.T) {
	err := WriteJSONL(failWriter{}, []Task{{ID: "a", Title: "t"}})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestReadJSONLBlankLines(t *testing.T) {
	// leading/trailing whitespace and blank lines are skipped.
	in := "  \n\t\n{\"id\":\"a\",\"title\":\"t\"}\n   \n"
	ts, err := ReadJSONL(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 || ts[0].ID != "a" {
		t.Fatalf("got %d tasks: %+v", len(ts), ts)
	}
}
