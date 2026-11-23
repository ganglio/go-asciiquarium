// SPDX-License-Identifier: GPL-2.0-or-later

//go:build badger2350

// Badger is the firmware for the Pimoroni Badger 2350. Build it with
//
//	tinygo build -target=badger2350 -o bin/aquarium.uf2 ./cmd/badger
//
// or with "make badger". Hold the badge's boot button while plugging it in,
// copy the .uf2 file to the drive that appears, or run "make flash".
//
// The badge shows the tank and sleeps. Every minute it wakes up, lets the tank
// live for that long and shows it, so the fish is somewhere else. Pressing
// button A drops food: the fish goes for it, the screen refreshes as fast as it
// can while that goes on and for half a minute more, and then it is back to one
// picture a minute. At night, from 22:00 to 07:00 by the real-time clock, the
// fish is gone, the picture is in negative, and there is one every ten minutes.
package main

import (
	"machine"

	"go-asciiquarium/internal/app"
	"go-asciiquarium/internal/epd"
)

// rngBits is the size of what the hardware random number generator returns.
const rngBits = 32

// Settings that replace parts of the default pace, set by "make" with
// -ldflags "-X main.paceIdle=2m ...", for instance make flash IDLE=2m
// NIGHT=23:00-06:30. Empty means the default.
var paceIdle, paceTail, paceNight, paceNightInterval string

// frame is the picture being drawn. It is a variable of its own, not part of
// the stack, because it is a few kilobytes.
var frame epd.Frame

func main() {
	hw := newHardware()

	// The hardware random number generator gives another number on every
	// boot, so every boot gets another tank.
	seed := func() uint64 {
		hi, _ := machine.GetRNG()
		lo, _ := machine.GetRNG()
		return uint64(hi)<<rngBits | uint64(lo)
	}
	tank := app.New(seed(), seed())

	display := epd.New(hw, epd.Config{})

	pace, err := app.DefaultPace.With(app.Overrides{Idle: paceIdle, Tail: paceTail, Night: paceNight, NightInterval: paceNightInterval})
	if err != nil {
		println("pace:", err.Error(), "- using the default one")
		pace = app.DefaultPace
	}

	tank.Run(hw, pace, func(kind app.Kind) {
		tank.Render(&frame)
		display.Show(&frame, kind == app.Settle)
	})
}
