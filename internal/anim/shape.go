// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import "strings"

// lines splits s at newlines the way Perl's split("\n", s) does: interior
// empty lines are kept, trailing empty lines are dropped.
func lines(s string) []string {
	parts := strings.Split(s, "\n")
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// buildFrames turns multi-line strings (one per animation frame) into grids
// of cells. Height and width are the maxima over all frames.
func buildFrames(frames []string) (grids [][][]rune, height, width int) {
	grids = make([][][]rune, len(frames))
	for i, s := range frames {
		rows := lines(s)
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
