// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import (
	"strings"
	"unicode"
)

// Lines splits s at newlines the way Perl's split("\n", s) does: interior
// empty lines are kept, trailing empty lines are dropped.
func Lines(s string) []string {
	parts := strings.Split(s, "\n")
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// buildFrames turns multi-line strings (one per animation frame) into grids
// of cells. A single newline at the start of each string is stripped, so
// literals can start on the line after their opening quote. Height and width
// are the maxima over all frames.
func buildFrames(frames []string) (grids [][][]rune, height, width int) {
	grids = make([][][]rune, len(frames))
	for i, s := range frames {
		s = strings.TrimPrefix(s, "\n")
		rows := Lines(s)
		grid := make([][]rune, len(rows))
		for r, line := range rows {
			grid[r] = []rune(line)
			width = max(width, len(grid[r]))
		}
		grids[i] = grid
		height = max(height, len(rows))
	}
	return grids, height, width
}

// autoTrans replaces the whitespace at the start of every line with the
// transparent character, so that the empty margin of a sprite does not
// overwrite what is behind it while spaces inside the sprite stay opaque.
func autoTrans(s string, trans rune) string {
	var b strings.Builder
	for _, line := range Lines(s) {
		runes := []rune(line)
		n := 0
		for n < len(runes) && unicode.IsSpace(runes[n]) {
			n++
		}
		for i := 0; i < n; i++ {
			runes[i] = trans
		}
		b.WriteString(string(runes))
		b.WriteByte('\n')
	}
	return b.String()
}
