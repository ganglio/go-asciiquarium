// SPDX-License-Identifier: GPL-2.0-or-later

package epd

import "time"

// Sim is a Transport without a screen behind it, for running the code on a
// computer: it counts the refreshes and takes as long over each as the real
// panel does.
type Sim struct {
	// RefreshTime is how long the screen is busy after an update starts.
	RefreshTime time.Duration

	// Refreshes counts the updates the screen was told to do.
	Refreshes int

	busy bool
}

func (s *Sim) Reset() {}

func (s *Sim) Command(cmd byte, _ ...byte) {
	if cmd == regActivate {
		s.Refreshes++
		s.busy = true
	}
}

func (s *Sim) Data(...byte) {}

func (s *Sim) WaitIdle() {
	if s.busy {
		time.Sleep(s.RefreshTime)
		s.busy = false
	}
}
