// SPDX-License-Identifier: GPL-2.0-or-later

package button

import (
	"testing"
	"time"
)

// pin is a button that the test presses and releases, in a time that it moves.
type pin struct {
	now  time.Time
	down bool
}

func newPin() (*pin, *Button) {
	p := &pin{now: time.Unix(1000, 0)}
	return p, New(func() bool { return p.down }, func() time.Time { return p.now })
}

func (p *pin) wait(d time.Duration) { p.now = p.now.Add(d) }

const look = 5 * time.Millisecond // how often it is looked at, in these tests

func TestAPressIsSeenOnce(t *testing.T) {
	p, b := newPin()
	if b.Pressed() {
		t.Fatal("pressed before anything happened")
	}
	p.down = true
	if !b.Pressed() {
		t.Fatal("the press was not seen")
	}
	if b.Pressed() {
		t.Error("the same press was seen twice")
	}
}

func TestAButtonHeldDownIsOnePress(t *testing.T) {
	p, b := newPin()
	p.down = true
	presses := 0
	for range 500 {
		if b.Pressed() {
			presses++
		}
		p.wait(look)
	}
	if presses != 1 {
		t.Errorf("%d presses while it was held down", presses)
	}
}

// The reason for it all: a press that comes and goes between two asks, as it
// does while the screen refreshes, is not forgotten, as long as it was looked
// at while it lasted.
func TestAShortPressSeenByLookIsRememberedUntilAsked(t *testing.T) {
	p, b := newPin()
	p.wait(time.Second)
	p.down = true
	b.Look() // for instance while waiting for the screen
	p.wait(100 * time.Millisecond)
	p.down = false
	b.Look()
	p.wait(2 * time.Second) // the refresh goes on, and nobody asks
	if !b.Pressed() {
		t.Fatal("the press was forgotten")
	}
	if b.Pressed() {
		t.Error("and then it was remembered twice")
	}
}

func TestBouncingIsOnePress(t *testing.T) {
	p, b := newPin()
	p.wait(time.Second)
	// Down, up, down, up, down in a few milliseconds, then held.
	for _, down := range []bool{true, false, true, false, true, true, true} {
		p.down = down
		b.Look()
		p.wait(time.Millisecond)
	}
	presses := 0
	if b.Pressed() {
		presses++
	}
	for range 100 {
		p.wait(look)
		if b.Pressed() {
			presses++
		}
	}
	if presses != 1 {
		t.Errorf("%d presses from a bounce", presses)
	}
}

func TestReleasingMayBounceToo(t *testing.T) {
	p, b := newPin()
	p.wait(time.Second)
	p.down = true
	b.Look()
	p.wait(120 * time.Millisecond) // held
	if !b.Pressed() {
		t.Fatal("the press was not seen")
	}
	// Let go: up, down for a moment, up again.
	for _, down := range []bool{false, true, false} {
		p.down = down
		b.Look()
		p.wait(2 * time.Millisecond)
	}
	p.wait(time.Second)
	if b.Pressed() {
		t.Error("the bounce of letting go was taken for another press")
	}
}

func TestTwoPressesAreTwoPresses(t *testing.T) {
	p, b := newPin()
	presses := 0
	for range 3 {
		p.wait(200 * time.Millisecond) // up for a while
		p.down = true
		if b.Pressed() {
			presses++
		}
		p.wait(80 * time.Millisecond)
		p.down = false
		b.Look()
	}
	if presses != 3 {
		t.Errorf("%d presses of 3", presses)
	}
}

func TestAPressAtOnceAfterTheLastIsNotSeen(t *testing.T) {
	// Debounce is the shortest time between letting go and pressing again.
	p, b := newPin()
	p.wait(time.Second)
	p.down = true
	b.Pressed()
	p.wait(50 * time.Millisecond)
	p.down = false
	b.Look()
	p.wait(Debounce / 2)
	p.down = true
	if b.Pressed() {
		t.Errorf("a press %v after letting go counted", Debounce/2)
	}
	p.down = false
	b.Look()
	p.wait(Debounce)
	p.down = true
	if !b.Pressed() {
		t.Errorf("a press %v after letting go did not count", Debounce)
	}
}
