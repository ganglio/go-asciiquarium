// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"go-asciiquarium/internal/anim"
)

func TestStyle(t *testing.T) {
	cyan := anim.Color{Hue: anim.Cyan}
	brightRed := anim.Color{Hue: anim.Red, Bold: true}

	// With colors: the ANSI palette, the bright half for bold, on black.
	for _, tt := range []struct {
		c    anim.Color
		want int
	}{{cyan, 6}, {brightRed, 9}} {
		fg, bg, _ := style(tt.c, false).Decompose()
		if fg != tcell.PaletteColor(tt.want) || bg != tcell.ColorBlack {
			t.Errorf("style(%v) = %v on %v, want palette color %d on black", tt.c, fg, bg, tt.want)
		}
	}

	// Without colors: nothing but the terminal's defaults, whatever the color.
	for _, c := range []anim.Color{cyan, brightRed, {}} {
		if got := style(c, true); got != tcell.StyleDefault {
			t.Errorf("mono style(%v) = %v, want the default style", c, got)
		}
	}
}
