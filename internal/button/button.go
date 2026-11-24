// SPDX-License-Identifier: GPL-2.0-or-later

// Package button turns what a push button reads into presses.
//
// A button is looked at now and then, not all the time, and a press can come
// and go between two looks: while the screen refreshes nothing else is going on
// to look. So the button remembers what happened at every look until somebody
// asks. Its contacts also bounce, reading down and up several times for a few
// milliseconds when pressed or released, and that must not be several presses.
package button

import "time"

// Debounce is how long a button has to have been up for its going down to be a
// new press. A contact bounces for a few milliseconds, and so does it on
// release, which can read as down again briefly; none of that is a press.
const Debounce = 30 * time.Millisecond

// Button is a push button, seen through a function that says whether it is
// down.
type Button struct {
	down func() bool
	now  func() time.Time

	isDown  bool      // it was down at the last look
	upSince time.Time // when it was last seen to come up
	pending bool      // a press nobody has asked about yet
}

// New makes a button out of a function that says whether it is down at the
// moment, and a clock.
func New(down func() bool, now func() time.Time) *Button {
	return &Button{down: down, now: now}
}

// Look reads the button, and remembers if it was pressed since the last look.
// Call it whenever there is a chance: that is what makes a short press count.
func (b *Button) Look() {
	down := b.down()
	switch {
	case down && !b.isDown:
		b.isDown = true
		if b.now().Sub(b.upSince) >= Debounce {
			b.pending = true
		}
	case !down && b.isDown:
		b.isDown = false
		b.upSince = b.now()
	}
}

// Pressed reports whether the button was pressed since the last time it was
// asked, and forgets it. A button held down is one press.
func (b *Button) Pressed() bool {
	b.Look()
	pressed := b.pending
	b.pending = false
	return pressed
}
