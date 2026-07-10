// Package buildinfo exposes build metadata, injected at link time via -ldflags
// (see the Makefile / goreleaser config). Falls back to Go's embedded VCS info
// when built with `go build` directly.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// These are overridden at build time:
//
//	-ldflags "-X .../internal/buildinfo.Version=v1.2.3 -X .../buildinfo.Commit=abc -X .../buildinfo.Date=..."
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

func init() {
	if Commit != "" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				Commit = s.Value
			case "vcs.time":
				if Date == "" {
					Date = s.Value
				}
			}
		}
	}
}

// Short returns a one-line version string.
func Short() string {
	c := Commit
	if len(c) > 12 {
		c = c[:12]
	}
	if c == "" {
		return Version
	}
	return fmt.Sprintf("%s (%s)", Version, c)
}

// String returns a multi-line human-readable version block.
func String() string {
	return fmt.Sprintf("tasks %s\n  commit:  %s\n  built:   %s\n  go:      %s\n  os/arch: %s/%s",
		Version, orNone(Commit), orNone(Date), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Map returns build info as key/value pairs (for the /version endpoint).
func Map() map[string]string {
	return map[string]string{
		"version": Version,
		"commit":  Commit,
		"date":    Date,
		"go":      runtime.Version(),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
