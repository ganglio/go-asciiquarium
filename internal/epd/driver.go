// SPDX-License-Identifier: GPL-2.0-or-later

package epd

// The register sequence, the waveform and the way the picture is laid out in
// the controller's RAM are transcribed from the firmware of the Badger 2350
// (modules/c/ssd1680/ssd1680.cpp in https://github.com/pimoroni/badger2350),
// which is the only sequence known to work on this panel. That file is
//
//	MIT License, Copyright (c) 2026 Pimoroni Ltd
//
// see THIRD_PARTY_NOTICES.md for the full text.
//
// The firmware drives the panel in four grey levels; here only black and
// white are needed, and a black pixel is a 1 in both of the controller's RAM
// planes, a white pixel a 0 in both, as in that firmware.

// Transport is the hardware side of the display: an SPI bus and four pins.
type Transport interface {
	// Reset pulses the RESET pin low, long enough for the controller to reset.
	Reset()
	// Command sends a command byte (DC low) followed by its parameters
	// (DC high), all in one chip-select cycle.
	Command(cmd byte, params ...byte)
	// Data sends more data (DC high) in a chip-select cycle of its own.
	Data(data ...byte)
	// WaitIdle blocks while the BUSY pin is high.
	WaitIdle()
}

// SSD1680 registers.
const (
	regDriverOutput = 0x01 // set gate driver output
	regGateVoltage  = 0x03 // gate level voltage control
	regSourceVolt   = 0x04 // source driving voltage control
	regBooster      = 0x0C // booster soft start
	regEntryMode    = 0x11 // data entry mode
	regSoftReset    = 0x12
	regActivate     = 0x20 // activate display update
	regUpdateCtrl2  = 0x22 // display update control 2
	regRAMBW        = 0x24 // write RAM (black/white)
	regRAMRed       = 0x26 // write RAM (red, here the second grey plane)
	regVCOM         = 0x2C // write VCOM register
	regLUT          = 0x32 // write LUT register
	regEndOption    = 0x3F // option for LUT end
	regRAMXRange    = 0x44 // set RAM x start/end
	regRAMYRange    = 0x45 // set RAM y start/end
	regRAMXCounter  = 0x4E // set RAM x counter
	regRAMYCounter  = 0x4F // set RAM y counter
)

// The RAM window: 176 pixels across, in bytes, and 264 rows. The controller
// counts rows down from the last one, which is where the first column of the
// picture goes, and bytes up from the first.
const (
	ramXStart = 0
	ramXEnd   = bytesPerColumn - 1
	ramYStart = Width - 1
	ramYEnd   = 0

	// A row number is two bytes.
	ramYStartLow  = ramYStart & byteMax
	ramYStartHigh = ramYStart >> bitsPerByte
	ramYEndLow    = ramYEnd & byteMax
	ramYEndHigh   = ramYEnd >> bitsPerByte
)

// Values for the registers, from the firmware.
const (
	// scanDefault is the setting of the gate scan direction and interlacing.
	scanDefault = 0

	// entryXUpYDown: the x counter counts up, the y counter down. Bit 1 would
	// invert y and bit 2 swap x and y.
	entryXUpYDown = 0b001

	// updateOnce is the update sequence: enable clock and analog, run the
	// update, disable analog and clock.
	updateOnce = 0xC7

	endOption   = 0x22
	gateVoltage = 0x17
	vcom        = 0x28
)

// sourceVoltage is the value of the source driving voltage register.
var sourceVoltage = [...]byte{0x41, 0xAE, 0x32}

// The waveform is what the controller drives the pixels with during an
// update. This one is a single pass that takes about a second. It is loaded
// into the controller's LUT register, which has a fixed size, and is made of
// a table of voltages, a table of timings, and some settings.
const (
	lutLevels     = 5  // voltage levels
	lutPhases     = 12 // phases of each level
	lutGroups     = 12 // timing groups
	lutGroupSize  = 7  // bytes of a timing group
	lutUsedGroups = 3  // groups that are not all zeros
	lutConfigSize = 9  // frame rate and XON

	lutVoltageSize = lutLevels * lutPhases
	lutTimingSize  = lutGroups * lutGroupSize

	// lutSize is the size of the LUT register.
	lutSize = 153
)

// The tables must add up to the register: it does not compile if they don't.
const (
	_ = uint(lutVoltageSize + lutTimingSize + lutConfigSize - lutSize)
	_ = uint(lutSize - (lutVoltageSize + lutTimingSize + lutConfigSize))
)

// The tables. What they leave out is zero.
var (
	lutVoltage = [lutLevels][lutPhases]byte{
		{0x40, 0x68},
		{0xA0, 0x65},
		{0xA8, 0x65, 0x40},
		{0xAA, 0x65, 0x50},
		{}, // the last level is not used
	}
	lutTiming = [lutUsedGroups][lutGroupSize]byte{
		{0x02, 0x00, 0x00, 0x05, 0x0A},
		{0x19, 0x19, 0x00, 0x02},
		{0x05, 0x0A},
	}
	lutConfig = [lutConfigSize]byte{0x44, 0x42, 0x22, 0x22, 0x23, 0x32}
)

// lut is the waveform as the register takes it.
var lut = buildLUT()

func buildLUT() []byte {
	b := make([]byte, 0, lutSize)
	for _, level := range lutVoltage {
		b = append(b, level[:]...)
	}
	for _, group := range lutTiming {
		b = append(b, group[:]...)
	}
	b = append(b, make([]byte, (lutGroups-lutUsedGroups)*lutGroupSize)...)
	return append(b, lutConfig[:]...)
}

// What the display does when nothing says otherwise.
const (
	defaultCleanEvery = 10
	defaultSettleMin  = 3
)

// Config sets how often the display cleans away ghosting.
type Config struct {
	// CleanEvery is how many pictures are shown between two cleanings. Each
	// picture leaves a faint trace of the one before; cleaning flashes the
	// screen black and white. Default 10.
	CleanEvery int
	// SettleMin is how many pictures since the last cleaning make it worth
	// cleaning once a burst of updates is over (see Show). Default 3.
	SettleMin int
}

// Stats counts what the display did.
type Stats struct {
	Pictures int // pictures shown
	Skipped  int // pictures not shown because the screen already had them
	Cleans   int // times the screen was flashed black and white
}

// Display is the e-ink screen. It only changes the screen when there is
// something new to show.
type Display struct {
	t   Transport
	cfg Config

	ready bool  // the controller has been set up
	shown bool  // last holds what is on the screen
	last  Frame // what is on the screen
	spare Frame // scratch space for cleaning
	debt  int   // pictures shown since the last cleaning

	stats Stats
}

// New creates a display on a transport.
func New(t Transport, cfg Config) *Display {
	if cfg.CleanEvery <= 0 {
		cfg.CleanEvery = defaultCleanEvery
	}
	if cfg.SettleMin <= 0 {
		cfg.SettleMin = defaultSettleMin
	}
	// Nobody knows what was on the screen before: clean it first.
	return &Display{t: t, cfg: cfg, debt: cfg.CleanEvery}
}

// Stats says what the display did so far.
func (d *Display) Stats() Stats { return d.stats }

// Show puts a picture on the screen, and blocks while the screen refreshes.
//
// If the screen already has this picture nothing happens, unless final is
// set and enough pictures have gone by since the last cleaning: final marks
// the last picture of a burst of updates, after which the screen is left
// alone for a long time, so that is the moment to clean it. Show returns
// whether it touched the screen.
func (d *Display) Show(f *Frame, final bool) bool {
	same := d.shown && f.Equal(&d.last)
	clean := d.debt >= d.cfg.CleanEvery || (final && d.debt >= d.cfg.SettleMin)
	if same && !clean {
		d.stats.Skipped++
		return false
	}
	if !d.ready {
		d.setup()
	}
	if clean {
		d.spare.Fill()
		d.update(&d.spare) // black
		d.spare.Clear()
		d.update(&d.spare) // white
		d.debt = 0
		d.stats.Cleans++
	}
	d.update(f)
	d.last = *f
	d.shown = true
	d.debt++
	d.stats.Pictures++
	return true
}

// setup resets the controller and sets the size of the RAM window.
func (d *Display) setup() {
	t := d.t
	t.Reset()
	t.WaitIdle()

	t.Command(regSoftReset)
	t.WaitIdle()

	t.Command(regDriverOutput, ramYStartLow, ramYStartHigh, scanDefault)
	t.Command(regEntryMode, entryXUpYDown)
	t.Command(regRAMXRange, ramXStart, ramXEnd)
	t.Command(regRAMYRange, ramYStartLow, ramYStartHigh, ramYEndLow, ramYEndHigh)
	t.WaitIdle()
	d.ready = true
}

// writeLUT loads the waveform. It is sent again for every update, as the
// firmware does: first the voltages, then each timing group on its own, then
// the rest.
func (d *Display) writeLUT() {
	t := d.t
	t.Command(regLUT, lut[:lutVoltageSize]...)
	for group := range lutUsedGroups {
		start := lutVoltageSize + group*lutGroupSize
		t.Data(lut[start : start+lutGroupSize]...)
	}
	t.Data(lut[lutVoltageSize+lutUsedGroups*lutGroupSize:]...)

	t.Command(regEndOption, endOption)
	t.Command(regGateVoltage, gateVoltage)
	t.Command(regSourceVolt, sourceVoltage[:]...)
	t.Command(regVCOM, vcom)
	t.WaitIdle()
}

// update sends a picture to the controller and refreshes the screen.
func (d *Display) update(f *Frame) {
	t := d.t
	t.WaitIdle() // a previous update may still be going

	d.writeLUT()

	for _, plane := range []byte{regRAMRed, regRAMBW} {
		t.Command(regRAMXCounter, ramXStart)
		t.Command(regRAMYCounter, ramYStartLow, ramYStartHigh)
		t.Command(plane)
		t.Data(f.Native()...)
	}

	// The firmware sends the booster command without parameters.
	t.Command(regBooster)
	t.Command(regUpdateCtrl2, updateOnce)
	t.WaitIdle()
	t.Command(regActivate)
	t.WaitIdle()
}
