// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"go-asciiquarium/internal/app"
)

// statusWidth is wide enough for the whole status line.
const statusWidth = 200

// statusLine reads the status line off a simulated screen.
func statusLine(s tcell.SimulationScreen) string {
	cells, width, _ := s.GetContents()
	var line strings.Builder
	for x := range width {
		if c := cells[statusRow*width+x]; len(c.Runes) > 0 {
			line.WriteRune(c.Runes[0])
		}
	}
	return line.String()
}

// reaction is how long the view has to react to a key: a few frames, but less
// than the second that a key which only set the clock to just before the
// change would take.
const reaction = 600 * time.Millisecond

// within waits for the status line to say something, for a while.
func within(t *testing.T, s tcell.SimulationScreen, what, why string) {
	t.Helper()
	deadline := time.Now().Add(reaction)
	for time.Now().Before(deadline) {
		if strings.Contains(statusLine(s), what) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: the status line never said %q, it says %q", why, what, strings.TrimSpace(statusLine(s)))
}

// TestTheKeysOfTheRealTimeView drives the real view in a simulated terminal:
// the keys n and d show the night and the day at once, f at night is a peek
// that says so, and q ends it.
func TestTheKeysOfTheRealTimeView(t *testing.T) {
	s := tcell.NewSimulationScreen("")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(statusWidth, statusRow+1)

	noon, err := clockOffset("12:00:00", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		showLive(s, app.New(7, 8), app.DefaultPace.Night, 7, noon)
		close(done)
	}()
	press := func(r rune) { s.InjectKey(tcell.KeyRune, r, tcell.ModNone) }

	within(t, s, "| day |", "at noon")
	press('n')
	within(t, s, "night, f peeks", "after n")
	press('f')
	within(t, s, "peeking at the day", "after f at night")
	press('d')
	within(t, s, "| day |", "after d")
	press('n')
	within(t, s, "night, f peeks", "after n again")

	press('q')
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("q did not end it")
	}
}
