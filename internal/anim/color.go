// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import "strings"

// Hue is one of the eight basic terminal colors, numbered like the ANSI
// palette so that a Hue can be used directly as a palette index.
type Hue uint8

const (
	Black Hue = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Color is a hue plus the "bold" flag. Term::Animation draws upper-case
// colors (e.g. 'R', "RED") in bold, which terminals show as the bright variant.
type Color struct {
	Hue  Hue
	Bold bool
}

var hueByLetter = map[string]Hue{
	"k": Black, "r": Red, "g": Green, "y": Yellow,
	"b": Blue, "m": Magenta, "c": Cyan, "w": White,
}

var hueByName = map[string]Hue{
	"black": Black, "red": Red, "green": Green, "yellow": Yellow,
	"blue": Blue, "magenta": Magenta, "cyan": Cyan, "white": White,
}

// ParseColor understands the color ids used by Term::Animation: a single
// letter (k r g y b m c w) or a full name (black, red, ...). Upper case means
// bold. Mixed case such as "Blue" is rejected.
func ParseColor(s string) (Color, bool) {
	lower := strings.ToLower(s)
	bold := s == strings.ToUpper(s)
	if s == "" || (s != lower && !bold) {
		return Color{}, false
	}
	if h, ok := hueByLetter[lower]; ok {
		return Color{Hue: h, Bold: bold}, true
	}
	if h, ok := hueByName[lower]; ok {
		return Color{Hue: h, Bold: bold}, true
	}
	return Color{}, false
}
