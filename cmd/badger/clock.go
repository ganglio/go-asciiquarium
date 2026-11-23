// SPDX-License-Identifier: GPL-2.0-or-later

//go:build badger2350

package main

import (
	"machine"
	"time"

	"go-asciiquarium/internal/rtc"
)

const (
	// i2cFrequency is the speed of the I2C bus, which Pimoroni's firmware uses
	// for the clock too.
	i2cFrequency = 400 * machine.KHz

	// i2cPowerUpDelay is how long to wait after switching on the power of the
	// I2C bus before using it, as Pimoroni's firmware does.
	i2cPowerUpDelay = 500 * time.Millisecond

	// buildTimeFormat is how buildTime is written: the time on the wall.
	buildTimeFormat = "2006-01-02T15:04:05"
)

// buildTime is when the firmware was built, as the time on the wall of the
// computer that built it, set by "make" with -ldflags "-X main.buildTime=...".
// The clock is set to it when it has lost its power or is behind it, which is
// how it gets its time: flash the badge and it is right to within a few
// minutes. It is empty if the firmware was not built with make.
var buildTime string

// setUpClock starts the I2C bus and the clock on it, and sets the clock if it
// needs setting. If it can't, the badge goes on without a clock and so without
// a night.
func (h *hardware) setUpClock() {
	err := machine.I2C0.Configure(machine.I2CConfig{
		Frequency: i2cFrequency,
		SDA:       machine.I2C0_SDA_PIN,
		SCL:       machine.I2C0_SCL_PIN,
	})
	if err != nil {
		println("clock: no I2C bus:", err.Error())
		return
	}
	time.Sleep(i2cPowerUpDelay)
	h.clock = rtc.New(machine.I2C0)

	built, err := time.Parse(buildTimeFormat, buildTime)
	if err != nil {
		return // not built with make: leave the clock alone
	}
	if now, err := h.clock.Now(); err != nil || now.Before(built) {
		if err := h.clock.Set(built); err != nil {
			println("clock: could not set it:", err.Error())
		}
	}
}

// Clock is the time on the wall, from the real-time clock. If the clock can't
// be read there is no way to tell, and the tank never goes to sleep.
func (h *hardware) Clock() (time.Time, bool) {
	if h.clock == nil {
		return time.Time{}, false
	}
	now, err := h.clock.Now()
	return now, err == nil
}
