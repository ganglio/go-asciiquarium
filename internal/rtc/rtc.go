// SPDX-License-Identifier: GPL-2.0-or-later

// Package rtc reads and sets the real-time clock of the Badger 2350, a
// PCF85063A on the I2C bus. The clock keeps the date and the time on a crystal
// of its own, and goes on while the main chip sleeps.
//
// The registers and the way to read and set the time are those of the chip's
// datasheet, as the Linux kernel's driver for it (drivers/rtc/rtc-pcf85063.c)
// uses them. The clock holds the time on the wall, with no time zone, and so
// does this package: the times it gives and takes are in UTC for lack of a
// better word, and mean "whatever the clock on the wall said".
package rtc

import (
	"errors"
	"time"
)

// Address is the I2C address of the clock.
const Address = 0x51

// Registers. The time is seven registers in a row, from the seconds to the
// year, in BCD: the tens in the high four bits and the units in the low four.
const (
	regControl1   = 0x00
	regSeconds    = 0x04
	timeRegisters = 7 // seconds, minutes, hours, day, weekday, month, year

	// control1Stop stops the clock, so that the time can be set without it
	// moving on in the middle. Everything else in the register is left at its
	// default, which is 24-hour mode.
	control1Stop = 1 << 5

	// secondsPowerLost is a bit of the seconds register: the clock stopped
	// since the time was last set, because it lost its power, and the time
	// it holds can't be trusted.
	secondsPowerLost = 1 << 7

	secondsMask = 0x7F
	minutesMask = 0x7F
	hoursMask   = 0x3F // in 24-hour mode
	dayMask     = 0x3F
	weekdayMask = 0x07
	monthMask   = 0x1F

	// The year register holds two digits, the years since yearBase.
	yearBase = 2000
	yearMax  = yearBase + 99

	bcdBase  = 10
	bcdShift = 4
	bcdUnits = 1<<bcdShift - 1
)

// ErrNotSet is returned when the clock has lost its power: it must be set.
var ErrNotSet = errors.New("rtc: the clock lost its power, so its time is not valid")

// ErrInvalid is returned when the clock holds a date that does not exist.
var ErrInvalid = errors.New("rtc: the clock holds a date that does not exist")

// ErrYear is returned when asked to set a year the clock can't hold.
var ErrYear = errors.New("rtc: the clock holds the years 2000 to 2099")

// Bus is the I2C bus the clock is on. machine.I2C has these methods.
type Bus interface {
	ReadRegister(address uint8, register uint8, data []byte) error
	WriteRegister(address uint8, register uint8, data []byte) error
}

// Clock is the real-time clock.
type Clock struct{ bus Bus }

// New returns the clock on a bus.
func New(bus Bus) *Clock { return &Clock{bus: bus} }

// Now is the time the clock has. It is ErrNotSet if the clock lost its power
// since it was set.
func (c *Clock) Now() (time.Time, error) {
	// The registers don't move on while they are being read, so all of them
	// must be read in one go, and within a second.
	var regs [timeRegisters]byte
	if err := c.bus.ReadRegister(Address, regSeconds, regs[:]); err != nil {
		return time.Time{}, err
	}
	return decode(regs)
}

// Set sets the clock to a time on the wall.
func (c *Clock) Set(t time.Time) error {
	regs, err := encode(t)
	if err != nil {
		return err
	}
	// Stop the clock, write the time, and let it run again.
	if err := c.bus.WriteRegister(Address, regControl1, []byte{control1Stop}); err != nil {
		return err
	}
	if err := c.bus.WriteRegister(Address, regSeconds, regs[:]); err != nil {
		return err
	}
	return c.bus.WriteRegister(Address, regControl1, []byte{0})
}

func toBCD(n int) byte { return byte(n/bcdBase<<bcdShift | n%bcdBase) }

// fromBCD is the number in a BCD byte, and whether it is one: neither digit
// can be more than 9.
func fromBCD(b byte) (int, bool) {
	tens, units := int(b>>bcdShift), int(b&bcdUnits)
	return tens*bcdBase + units, tens < bcdBase && units < bcdBase
}

func decode(regs [timeRegisters]byte) (time.Time, error) {
	if regs[0]&secondsPowerLost != 0 {
		return time.Time{}, ErrNotSet
	}
	var fields [timeRegisters]int
	masks := [timeRegisters]byte{secondsMask, minutesMask, hoursMask, dayMask, weekdayMask, monthMask, 0xFF}
	for i, reg := range regs {
		n, ok := fromBCD(reg & masks[i])
		if i == 4 { // the weekday is a number, not BCD
			n, ok = int(reg&weekdayMask), true
		}
		if !ok {
			return time.Time{}, ErrInvalid
		}
		fields[i] = n
	}
	sec, min, hour, day, month, year := fields[0], fields[1], fields[2], fields[3], fields[5], yearBase+fields[6]
	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	// time.Date makes a date out of anything, including the 31st of February.
	if sec >= 60 || min >= 60 || hour >= 24 || month < 1 || month > 12 || t.Day() != day {
		return time.Time{}, ErrInvalid
	}
	return t, nil
}

func encode(t time.Time) ([timeRegisters]byte, error) {
	if t.Year() < yearBase || t.Year() > yearMax {
		return [timeRegisters]byte{}, ErrYear
	}
	return [timeRegisters]byte{
		toBCD(t.Second()), // this also clears the power-lost bit
		toBCD(t.Minute()),
		toBCD(t.Hour()),
		toBCD(t.Day()),
		byte(t.Weekday()),
		toBCD(int(t.Month())),
		toBCD(t.Year() - yearBase),
	}, nil
}
