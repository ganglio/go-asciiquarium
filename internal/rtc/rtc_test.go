// SPDX-License-Identifier: GPL-2.0-or-later

package rtc

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// chip is a fake PCF85063A: a row of registers that a bus reads and writes,
// with the address moving on after each byte, as on the real one.
type chip struct {
	regs   [0x12]byte
	log    []string
	failOn string // "read" or "write": make the bus fail
}

// newChip is a chip that has just lost its power.
func newChip() *chip {
	c := &chip{}
	c.regs[regSeconds] = secondsPowerLost
	return c
}

var errBus = errors.New("bus error")

func (c *chip) ReadRegister(address uint8, register uint8, data []byte) error {
	if c.failOn == "read" {
		return errBus
	}
	if address != Address {
		return errors.New("wrong address")
	}
	c.log = append(c.log, "read")
	copy(data, c.regs[register:])
	return nil
}

func (c *chip) WriteRegister(address uint8, register uint8, data []byte) error {
	if c.failOn == "write" {
		return errBus
	}
	if address != Address {
		return errors.New("wrong address")
	}
	copy(c.regs[register:], data)
	switch {
	case register == regControl1 && data[0]&control1Stop != 0:
		c.log = append(c.log, "stop")
	case register == regControl1:
		c.log = append(c.log, "run")
	case register == regSeconds:
		c.log = append(c.log, "time")
	}
	return nil
}

func at(y int, mo time.Month, d, h, mi, s int) time.Time {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
}

func TestAClockThatLostItsPowerMustBeSet(t *testing.T) {
	if _, err := New(newChip()).Now(); !errors.Is(err, ErrNotSet) {
		t.Errorf("err = %v, want ErrNotSet", err)
	}
}

func TestSetThenNow(t *testing.T) {
	for _, want := range []time.Time{
		at(2026, time.November, 23, 14, 3, 9),
		at(2000, time.January, 1, 0, 0, 0),
		at(2024, time.February, 29, 23, 59, 59), // a leap day
		at(2099, time.December, 31, 23, 59, 59),
		at(2026, time.July, 4, 12, 0, 0),
		at(2026, time.October, 10, 9, 8, 7), // digits of both kinds
	} {
		c := New(newChip())
		if err := c.Set(want); err != nil {
			t.Fatalf("Set(%v): %v", want, err)
		}
		got, err := c.Now()
		if err != nil || !got.Equal(want) {
			t.Errorf("Set(%v), then Now() = %v, %v", want, got, err)
		}
	}
}

func TestSettingStopsTheClockWritesTheTimeAndRestartsIt(t *testing.T) {
	chip := newChip()
	if err := New(chip).Set(at(2026, time.November, 23, 14, 3, 9)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"stop", "time", "run"}; !slices.Equal(chip.log, want) {
		t.Errorf("sequence %v, want %v", chip.log, want)
	}
	if chip.regs[regSeconds]&secondsPowerLost != 0 {
		t.Error("the power-lost bit is still set after setting the time")
	}
	if chip.regs[regControl1] != 0 {
		t.Errorf("control register %#x after setting: not running in 24-hour mode", chip.regs[regControl1])
	}
}

func TestTheTimeIsStoredAsBCDInTheDocumentedRegisters(t *testing.T) {
	chip := newChip()
	New(chip).Set(at(2026, time.November, 23, 14, 3, 9)) // a Monday
	for i, want := range []byte{0x09, 0x03, 0x14, 0x23, 0x01, 0x11, 0x26} {
		if got := chip.regs[regSeconds+i]; got != want {
			t.Errorf("register %#x = %#x, want %#x", regSeconds+i, got, want)
		}
	}
}

func TestReadingIsOneBurstOfAllTheTimeRegisters(t *testing.T) {
	chip := newChip()
	New(chip).Set(at(2026, time.November, 23, 14, 3, 9))
	chip.log = nil
	New(chip).Now()
	if want := []string{"read"}; !slices.Equal(chip.log, want) {
		t.Errorf("%v: the registers must be read in one go", chip.log)
	}
}

func TestSettingRefusesYearsTheClockCantHold(t *testing.T) {
	for _, year := range []int{1999, 2100, 1970} {
		if err := New(newChip()).Set(at(year, time.June, 1, 0, 0, 0)); !errors.Is(err, ErrYear) {
			t.Errorf("year %d: err = %v, want ErrYear", year, err)
		}
	}
}

func TestADateThatDoesNotExistIsRefused(t *testing.T) {
	good := [timeRegisters]byte{0x09, 0x03, 0x14, 0x23, 0x01, 0x11, 0x26}
	for name, mutate := range map[string]func(*[timeRegisters]byte){
		"digit over 9 in the seconds": func(r *[timeRegisters]byte) { r[0] = 0x1A },
		"61 seconds":                  func(r *[timeRegisters]byte) { r[0] = 0x61 },
		"60 minutes":                  func(r *[timeRegisters]byte) { r[1] = 0x60 },
		"24 hours":                    func(r *[timeRegisters]byte) { r[2] = 0x24 },
		"day 0":                       func(r *[timeRegisters]byte) { r[3] = 0x00 },
		"the 31st of February":        func(r *[timeRegisters]byte) { r[3], r[5] = 0x31, 0x02 },
		"month 13":                    func(r *[timeRegisters]byte) { r[5] = 0x13 },
		"month 0":                     func(r *[timeRegisters]byte) { r[5] = 0x00 },
		"digit over 9 in the year":    func(r *[timeRegisters]byte) { r[6] = 0x2F },
	} {
		regs := good
		mutate(&regs)
		if _, err := decode(regs); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := decode(good); err != nil {
		t.Errorf("the good date was refused: %v", err)
	}
}

func TestBusErrorsAreReported(t *testing.T) {
	chip := newChip()
	chip.failOn = "read"
	if _, err := New(chip).Now(); !errors.Is(err, errBus) {
		t.Errorf("Now: %v", err)
	}
	chip.failOn = "write"
	if err := New(chip).Set(at(2026, time.January, 1, 0, 0, 0)); !errors.Is(err, errBus) {
		t.Errorf("Set: %v", err)
	}
}

func TestBCD(t *testing.T) {
	for n := 0; n < 100; n++ {
		got, ok := fromBCD(toBCD(n))
		if !ok || got != n {
			t.Errorf("%d -> %#x -> %d, %v", n, toBCD(n), got, ok)
		}
	}
	if _, ok := fromBCD(0x0A); ok {
		t.Error("0x0A is not BCD")
	}
	if _, ok := fromBCD(0xA0); ok {
		t.Error("0xA0 is not BCD")
	}
}
