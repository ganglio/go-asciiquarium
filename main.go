// SPDX-License-Identifier: GPL-2.0-or-later

// Asciiquarium is an aquarium/sea animation in ASCII art.
//
// This is a Go port of the Perl script by Kirk Baucom, version 1.1:
// http://robobunny.com/projects/asciiquarium
//
// Keys: q quit, r redraw (recreates every entity, picks up a new terminal
// size), p pause.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"go-asciiquarium/internal/anim"
	"go-asciiquarium/internal/aquarium"
)

// frameTime is the time between frames: the original waits 0.1s in getch.
const frameTime = 100 * time.Millisecond

func main() {
	classic := flag.Bool("c", false, `"classic" mode, only show species from Asciiquarium 1.0`)
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *version {
		fmt.Println("asciiquarium", aquarium.Version)
		return
	}

	if err := run(aquarium.Options{Classic: *classic}); err != nil {
		fmt.Fprintln(os.Stderr, "asciiquarium:", err)
		os.Exit(1)
	}
}

func run(opts aquarium.Options) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))

	// Leave the terminal in a sane state on SIGTERM/SIGHUP too.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	width, height := screen.Size()
	a := anim.New(width, height)
	rng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64()))
	scene := aquarium.New(a, opts, rng)
	canvas := tcellCanvas{screen}

	scene.Populate()

	events := make(chan tcell.Event)
	go func() {
		// PollEvent returns nil once the screen is finalized.
		for ev := screen.PollEvent(); ev != nil; ev = screen.PollEvent() {
			select {
			case events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	ticker := time.NewTicker(frameTime)
	defer ticker.Stop()
	paused := false

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if !paused {
				a.Animate()
			}
			a.Draw(canvas)
			screen.Show()

		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventResize:
				// Like the original, ignore resizes until the user asks for
				// a redraw. Just make the terminal repaint what is there.
				screen.Sync()

			case *tcell.EventKey:
				if ev.Key() == tcell.KeyCtrlC {
					return nil
				}
				if ev.Key() != tcell.KeyRune {
					continue
				}
				switch unicode.ToLower(ev.Rune()) {
				case 'q':
					return nil
				case 'p':
					paused = !paused
				case 'r':
					screen.Sync()
					width, height = screen.Size()
					a.Resize(width, height)
					a.RemoveAll()
					scene.Populate()
				}
			}
		}
	}
}

// tcellCanvas draws an animation on a tcell screen.
type tcellCanvas struct {
	screen tcell.Screen
}

func (c tcellCanvas) Clear() { c.screen.Clear() }

func (c tcellCanvas) SetCell(x, y int, r rune, col anim.Color) {
	c.screen.SetContent(x, y, r, nil, style(col))
}

// style maps an animation color to a terminal style on a black background.
// The original draws bold colors with A_BOLD; the bright half of the ANSI
// palette is what terminals show for that, without depending on the font.
func style(c anim.Color) tcell.Style {
	index := int(c.Hue)
	if c.Bold {
		index += 8
	}
	return tcell.StyleDefault.
		Foreground(tcell.PaletteColor(index)).
		Background(tcell.ColorBlack)
}
