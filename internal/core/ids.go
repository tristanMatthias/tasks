package core

import (
	"crypto/rand"
	"strconv"
	"strings"
)

const base36 = "0123456789abcdefghijklmnopqrstuvwxyz"

// randSuffix returns an n-char base36 string (beads-style, e.g. "07po").
func randSuffix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should not fail; fall back to a fixed-ish pattern.
		for i := range b {
			b[i] = byte(i)
		}
	}
	out := make([]byte, n)
	for i, c := range b {
		out[i] = base36[int(c)%len(base36)]
	}
	return string(out)
}

// DerivePrefix infers the issue prefix from a sample id by stripping the final
// "-<suffix>" token. "forge-crafting-intepreters-07po" -> "forge-crafting-intepreters".
// Child ids ("...-ps3t.2.1") also yield the same prefix. Returns "" if none.
func DerivePrefix(ids []string) string {
	for _, id := range ids {
		if strings.Contains(id, ":") { // synthetic memory ids
			continue
		}
		if i := strings.LastIndex(id, "-"); i > 0 {
			return id[:i]
		}
	}
	return ""
}

// nextChildIndex returns the next integer child suffix for a parent given the
// existing child ids. "<parent>.N" -> max(N)+1, starting at 1.
func nextChildIndex(parent string, childIDs []string) int {
	max := 0
	dotPrefix := parent + "."
	for _, c := range childIDs {
		if !strings.HasPrefix(c, dotPrefix) {
			continue
		}
		rest := c[len(dotPrefix):]
		// direct child only: first segment before any further dot
		if j := strings.IndexByte(rest, '.'); j >= 0 {
			rest = rest[:j]
		}
		if n, err := strconv.Atoi(rest); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}
