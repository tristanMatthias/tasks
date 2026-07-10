package model

// NaturalLess reports whether a sorts before b using human/natural ordering:
// digit runs compare numerically (so "ps3t.2" < "ps3t.11"), other runs compare
// lexically. Mirrors the UI's naturalCompare in web/static/app.js.
func NaturalLess(a, b string) bool { return naturalCompare(a, b) < 0 }

func naturalCompare(a, b string) int {
	ax := chunk(a)
	bx := chunk(b)
	n := len(ax)
	if len(bx) < n {
		n = len(bx)
	}
	for i := 0; i < n; i++ {
		as, bs := ax[i], bx[i]
		aNum := as[0] >= '0' && as[0] <= '9'
		bNum := bs[0] >= '0' && bs[0] <= '9'
		if aNum && bNum {
			if d := cmpNum(as, bs); d != 0 {
				return d
			}
		} else if as != bs {
			if as < bs {
				return -1
			}
			return 1
		}
	}
	return len(ax) - len(bx)
}

// chunk splits s into alternating runs of digits and non-digits.
func chunk(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		j := i
		isDigit := s[i] >= '0' && s[i] <= '9'
		for j < len(s) {
			d := s[j] >= '0' && s[j] <= '9'
			if d != isDigit {
				break
			}
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

// cmpNum compares two digit strings numerically without overflow by trimming
// leading zeros then comparing length, then lexically.
func cmpNum(a, b string) int {
	a = trimLeadingZeros(a)
	b = trimLeadingZeros(b)
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}
