// SPDX-License-Identifier: GPL-2.0-or-later

package epd

import (
	"fmt"

	"go-asciiquarium/internal/flip"
)

// glyphArt is the font: only the characters the aquarium uses, each in a
// 6x8 cell. '#' is a black pixel. Neighbouring cells touch, so the shapes
// leave a column free, except for '~', which has to join up with itself.
//
// A character that turns into another when mirrored (see package flip) is
// drawn here only if it is the first of its pair. The second is not drawn: it
// is the mirror image of the first, made when the font is compiled.
var glyphArt = map[rune][CellH]string{
	' ': {
		"......",
		"......",
		"......",
		"......",
		"......",
		"......",
		"......",
		"......",
	},
	// The water surface. Six pixels wide so that a row of them is one wave.
	'~': {
		"......",
		"......",
		"......",
		".##...",
		"#..#..",
		"....##",
		"......",
		"......",
	},
	// The algae, and the body of the fish. ')' is made from this.
	'(': {
		"...#..",
		"..#...",
		".#....",
		".#....",
		".#....",
		".#....",
		"..#...",
		"...#..",
	},
	// More brackets, so that a fish can be made of them. ']' and '}' are made
	// from these.
	'[': {
		".###..",
		".#....",
		".#....",
		".#....",
		".#....",
		".#....",
		".#....",
		".###..",
	},
	'{': {
		"..##..",
		"..#...",
		"..#...",
		".#....",
		".#....",
		"..#...",
		"..#...",
		"..##..",
	},
	// A bubble grows from '.' to 'o' to 'O'.
	'.': {
		"......",
		"......",
		"......",
		"......",
		"......",
		"......",
		"..##..",
		"..##..",
	},
	'o': {
		"......",
		"......",
		"......",
		"..##..",
		".#..#.",
		".#..#.",
		"..##..",
		"......",
	},
	'O': {
		"......",
		"..##..",
		".#..#.",
		"#....#",
		"#....#",
		".#..#.",
		"..##..",
		"......",
	},
	// A pellet of food.
	'*': {
		"......",
		"......",
		"..#...",
		"#.#.#.",
		".###..",
		"#.#.#.",
		"..#...",
		"......",
	},
	// The fish: the mouth and the tail, which are '<' and '>'; '>' is made from
	// this.
	'<': {
		"......",
		"....#.",
		"...#..",
		"..#...",
		".#....",
		"..#...",
		"...#..",
		"....#.",
	},
	'=': {
		"......",
		"......",
		"......",
		".####.",
		"......",
		".####.",
		"......",
		"......",
	},
	// The eye is a dot, not a masculine ordinal indicator.
	'º': {
		"......",
		"......",
		"..##..",
		".####.",
		".####.",
		"..##..",
		"......",
		"......",
	},
}

// missingGlyph is drawn for a character that is not in the font: an empty box.
var missingGlyph = [CellH]string{
	"......",
	".####.",
	".#..#.",
	".#..#.",
	".#..#.",
	".#..#.",
	".####.",
	"......",
}

// glyphs holds the font as bits: one byte per row, the leftmost pixel in
// bit CellW-1.
var glyphs = compileGlyphs()

var missing = compile('?', missingGlyph)

func compileGlyphs() map[rune][CellH]uint8 {
	out := make(map[rune][CellH]uint8, len(glyphArt)+len(flip.Pairs))
	for r, art := range glyphArt {
		out[r] = compile(r, art)
	}
	for _, pair := range flip.Pairs {
		drawn, made := pair[0], pair[1]
		art, ok := glyphArt[drawn]
		if !ok {
			panic(fmt.Sprintf("epd: %q has no glyph to make the glyph of %q from", drawn, made))
		}
		if _, ok := glyphArt[made]; ok {
			panic(fmt.Sprintf("epd: %q is drawn, but it is made from %q", made, drawn))
		}
		out[made] = compile(made, mirrorGlyph(art))
	}
	return out
}

func compile(r rune, art [CellH]string) [CellH]uint8 {
	var bits [CellH]uint8
	for y, row := range art {
		if len(row) != CellW {
			panic(fmt.Sprintf("epd: glyph %q row %d is %d pixels wide, want %d", r, y, len(row), CellW))
		}
		for x := 0; x < CellW; x++ {
			switch row[x] {
			case '#':
				bits[y] |= 1 << (CellW - 1 - x)
			case '.':
			default:
				panic(fmt.Sprintf("epd: glyph %q row %d has %q, want '#' or '.'", r, y, row[x]))
			}
		}
	}
	return bits
}

// HasGlyph reports whether the font has a character.
func HasGlyph(r rune) bool {
	_, ok := glyphs[r]
	return ok
}

func glyphFor(r rune) [CellH]uint8 {
	if g, ok := glyphs[r]; ok {
		return g
	}
	return missing
}

// mirrorGlyph is a glyph flipped from left to right around its own bounding
// box, so that it keeps its place in the cell and so its spacing.
func mirrorGlyph(art [CellH]string) [CellH]string {
	minX, maxX := CellW, -1
	for _, row := range art {
		for x := range row {
			if row[x] == '#' {
				minX, maxX = min(minX, x), max(maxX, x)
			}
		}
	}
	if maxX < 0 { // nothing drawn
		return art
	}
	var out [CellH]string
	for y, row := range art {
		flipped := []byte(row)
		for l, r := minX, maxX; l < r; l, r = l+1, r-1 {
			flipped[l], flipped[r] = flipped[r], flipped[l]
		}
		out[y] = string(flipped)
	}
	return out
}
