// SPDX-License-Identifier: GPL-2.0-or-later

// Package flip knows which characters turn into each other when a picture is
// mirrored from left to right: '<' into '>', '(' into ')', and so on.
//
// It is the one place that says so. The fish uses it to turn around, and the
// font uses it to draw only half of its brackets: the other half are the
// mirror images of the first.
package flip

import "slices"

// Pairs are the characters that swap places when mirrored. The first of each
// pair is the one the font draws; the second is made from it. Every character
// may only be in one pair.
var Pairs = [...][2]rune{
	{'<', '>'},
	{'(', ')'},
	{'[', ']'},
	{'{', '}'},
}

// partner says what each character of Pairs turns into.
var partner = func() map[rune]rune {
	m := make(map[rune]rune, 2*len(Pairs))
	for _, pair := range Pairs {
		m[pair[0]], m[pair[1]] = pair[1], pair[0]
	}
	return m
}()

// Rune is what a character turns into when mirrored: its partner, or itself
// if it looks the same from both sides, like 'o'.
func Rune(r rune) rune {
	if p, ok := partner[r]; ok {
		return p
	}
	return r
}

// String mirrors a line of characters: it reverses them and turns each
// directional one around.
func String(s string) string {
	r := []rune(s)
	slices.Reverse(r)
	for i, c := range r {
		r[i] = Rune(c)
	}
	return string(r)
}
