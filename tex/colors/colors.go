// Package colors resolves CSS named colors and hexadecimal RGB/RGBA values.
package colors

import (
	"golang.org/x/image/colornames"
	"image/color"
	"strconv"
	"strings"
)

func Parse(s string) (color.NRGBA, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "rebeccapurple" {
		return color.NRGBA{R: 102, G: 51, B: 153, A: 255}, true
	}
	if s == "transparent" {
		return color.NRGBA{}, true
	}
	if c, ok := colornames.Map[s]; ok {
		return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}, true
	}
	if !strings.HasPrefix(s, "#") {
		return color.NRGBA{}, false
	}
	s = s[1:]
	if len(s) == 3 || len(s) == 4 {
		var b strings.Builder
		for _, r := range s {
			b.WriteRune(r)
			b.WriteRune(r)
		}
		s = b.String()
	}
	if len(s) != 6 && len(s) != 8 {
		return color.NRGBA{}, false
	}
	n, e := strconv.ParseUint(s, 16, 32)
	if e != nil {
		return color.NRGBA{}, false
	}
	if len(s) == 6 {
		return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, true
	}
	return color.NRGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, true
}
