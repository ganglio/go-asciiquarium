// SPDX-License-Identifier: GPL-2.0-or-later

//go:build badger2350

package main

import (
	"machine"
	"time"

	"go-asciiquarium/internal/app"
	"go-asciiquarium/internal/rtc"
)

const (
	// spiFrequency is the speed of the screen's bus. The controller takes more,
	// but a picture is only about 12 KB and a refresh takes a second.
	spiFrequency = 4 * machine.MHz

	// spiMode is SPI mode 0: clock idle low, data read on the rising edge.
	spiMode = 0

	// resetPulse is how long the reset pin is held low, and then high again,
	// as in Pimoroni's firmware.
	resetPulse = 10 * time.Millisecond

	// busyRaiseDelay is how long to give the screen to raise BUSY after a
	// command, so that a refresh that has just been started is not mistaken
	// for one that is over.
	busyRaiseDelay = time.Millisecond

	// busyPollInterval is how often to look at BUSY while waiting for the screen.
	busyPollInterval = 5 * time.Millisecond

	// buttonPollInterval is how often to look at the button while idle. It is
	// also what keeps the processor asleep the rest of the time.
	buttonPollInterval = 20 * time.Millisecond
)

// hardware is the Badger 2350's screen bus and button. It is the
// epd.Transport of the display and the app.Env of the tank.
//
// UNTESTED ON A BADGE. The pins come from TinyGo's board file for the badge,
// which matches Pimoroni's board/pins.csv, and the way they are used from
// Pimoroni's firmware (modules/c/powman and modules/c/ssd1680 in
// https://github.com/pimoroni/badger2350). Before the first real flash, check
// the list in README.md.
type hardware struct {
	spi *machine.SPI

	clock *rtc.Clock // nil if the real-time clock could not be set up

	cs, dc, reset, busy machine.Pin

	feed button
}

func newHardware() *hardware {
	// Pimoroni's firmware drives this pin high at start-up. Its header calls
	// it "I2C power for talking to the RTC"; it is not used here.
	machine.POWER_EN.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.POWER_EN.High()

	h := &hardware{
		spi:   machine.SPI0,
		cs:    machine.INKY_CS,
		dc:    machine.INKY_DC,
		reset: machine.INKY_RES,
		busy:  machine.INKY_BUSY,
		feed:  button{pin: machine.BUTTON_A},
	}

	for _, p := range []machine.Pin{h.cs, h.dc, h.reset} {
		p.Configure(machine.PinConfig{Mode: machine.PinOutput})
	}
	h.cs.High()
	h.reset.High()
	h.busy.Configure(machine.PinConfig{Mode: machine.PinInput})

	// The screen only listens. Its BUSY line is on GPIO 16, the pin SPI0
	// would use for receiving, so tell the bus not to take it.
	err := h.spi.Configure(machine.SPIConfig{
		Frequency: spiFrequency,
		SCK:       machine.INKY_SCLK,
		SDO:       machine.INKY_MOSI,
		SDI:       machine.NoPin,
		Mode:      spiMode,
	})
	if err != nil {
		panic(err)
	}

	// The buttons are pulled up and read low when pressed.
	h.feed.pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	h.setUpClock()
	return h
}

// Reset pulses the reset pin low.
func (h *hardware) Reset() {
	h.reset.Low()
	time.Sleep(resetPulse)
	h.reset.High()
	time.Sleep(resetPulse)
}

// Command sends a command byte followed by its parameters.
func (h *hardware) Command(cmd byte, params ...byte) {
	h.cs.Low()
	h.dc.Low()
	h.spi.Tx([]byte{cmd}, nil)
	if len(params) > 0 {
		h.dc.High()
		h.spi.Tx(params, nil)
	}
	h.cs.High()
}

// Data sends more data for the last command.
func (h *hardware) Data(data ...byte) {
	h.cs.Low()
	h.dc.High()
	h.spi.Tx(data, nil)
	h.cs.High()
}

// WaitIdle waits while the screen is busy. The CPU sleeps in between checks.
func (h *hardware) WaitIdle() {
	time.Sleep(busyRaiseDelay)
	for h.busy.Get() {
		time.Sleep(busyPollInterval)
	}
}

// button is a push button that reads low when pressed.
type button struct {
	pin  machine.Pin
	down bool
}

// Pressed reports whether the button went down since the last call.
func (b *button) Pressed() bool {
	now := !b.pin.Get()
	pressed := now && !b.down
	b.down = now
	return pressed
}

// Now is the time since the badge started, which is all the tank needs.
func (h *hardware) Now() time.Time { return time.Now() }

// Wait sleeps for up to d, or until button A is pressed. It looks at the button
// every buttonPollInterval, and in between the scheduler idles the processor.
// That is light sleep, which is as far as TinyGo can sleep today: it has no
// support for the chip's deep sleep. A d of zero only looks at the button.
func (h *hardware) Wait(d time.Duration) app.Event {
	deadline := time.Now().Add(d)
	for {
		if h.feed.Pressed() {
			return app.Pressed
		}
		left := time.Until(deadline)
		if left <= 0 {
			return app.Timeout
		}
		time.Sleep(min(buttonPollInterval, left))
	}
}
