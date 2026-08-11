package main

import (
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	out, err := captureRun(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "tasks ") {
		t.Fatalf("version output missing build info: %q", out)
	}
}

func TestPrintHelpers(t *testing.T) {
	for _, s := range []string{"open", "in_progress", "closed", "deferred", "weird"} {
		if statusGlyph(s) == "" {
			t.Fatalf("statusGlyph(%q) empty", s)
		}
	}
	if orDash("") != "—" || orDash("x") != "x" {
		t.Fatal("orDash")
	}
	if dateOnly("2026-01-02T03:04:05Z") != "2026-01-02" || dateOnly("short") != "short" {
		t.Fatal("dateOnly")
	}
	if dateTime("2026-01-02T03:04:05Z") != "2026-01-02 03:04" || dateTime("2026-01-02") != "2026-01-02" {
		t.Fatal("dateTime")
	}
}

func TestSelfUpdateHelpViaRun(t *testing.T) {
	// Routes through Run's self-update dispatch; --help returns before any network.
	if err := Run([]string{"self-update", "--help"}); err != nil {
		t.Fatalf("self-update --help: %v", err)
	}
	if err := Run([]string{"upgrade", "--bogus"}); err == nil {
		t.Fatal("upgrade alias should reject an unknown flag")
	}
}
