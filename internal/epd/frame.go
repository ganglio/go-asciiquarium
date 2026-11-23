// SPDX-License-Identifier: GPL-2.0-or-later

// Package epd drives the 2.7" e-ink screen of the Pimoroni Badger 2350 (an
// SSD1680 controller, 264x176 pixels). It has nothing platform-specific in
// it: the pins and the SPI bus come in through the Transport interface, so
// the same code runs on the badge and, with a fake transport, in tests.
package epd

// The screen is 264 pixels wide and 176 tall. Characters are drawn as 6x8
// pixel cells, which makes a grid of 44 columns by 22 rows.
const (
	Width  = 264
	Height = 176

	CellW = 6
	CellH = 8
	Cols  = Width / CellW
	Rows  = Height / CellH

	// A byte holds 8 pixels, one per bit, the topmost in the highest bit.
	bitsPerByte = 8
	byteMax     = 1<<bitsPerByte - 1
	topBit      = 1 << (bitsPerByte - 1)

	// The controller takes the picture one column of 8-pixel-high bytes at a
	// time: Height/bitsPerByte bytes for each of the Width columns.
	bytesPerColumn = Height / bitsPerByte
)

// Frame is a black-and-white picture, stored the way the controller's RAM
// wants it so that it can be sent as it is: column by column, from the left,
// each column as bytesPerColumn bytes with the topmost pixel in the highest bit.
// A set bit is a black pixel; a new Frame is all white.
type Frame struct {
	native [Width * bytesPerColumn]byte
}

// Clear makes the whole frame white.
func (f *Frame) Clear() { clear(f.native[:]) }

// Fill makes the whole frame black.
func (f *Frame) Fill() {
	for i := range f.native {
		f.native[i] = byteMax
	}
}

// Invert swaps black and white everywhere.
func (f *Frame) Invert() {
	for i := range f.native {
		f.native[i] ^= byteMax
	}
}

// Set colors a pixel. Pixels outside the screen are ignored.
func (f *Frame) Set(x, y int, black bool) {
	if x < 0 || x >= Width || y < 0 || y >= Height {
		return
	}
	i, bit := position(x, y)
	if black {
		f.native[i] |= bit
	} else {
		f.native[i] &^= bit
	}
}

// Black reports whether a pixel is black. Pixels outside the screen are white.
func (f *Frame) Black(x, y int) bool {
	if x < 0 || x >= Width || y < 0 || y >= Height {
		return false
	}
	i, bit := position(x, y)
	return f.native[i]&bit != 0
}

// position says where a pixel is: the byte, and the bit in it.
func position(x, y int) (index int, bit byte) {
	return x*bytesPerColumn + y/bitsPerByte, topBit >> (y % bitsPerByte)
}

// Equal reports whether two frames are the same picture.
func (f *Frame) Equal(o *Frame) bool { return f.native == o.native }

// Native is the picture in the order the controller takes it.
func (f *Frame) Native() []byte { return f.native[:] }

// DrawGlyph draws a character in the cell at the given column and row,
// replacing whatever was there.
func (f *Frame) DrawGlyph(col, row int, r rune) {
	if col < 0 || col >= Cols || row < 0 || row >= Rows {
		return
	}
	g := glyphFor(r)
	for py := 0; py < CellH; py++ {
		for px := 0; px < CellW; px++ {
			f.Set(col*CellW+px, row*CellH+py, g[py]&(1<<(CellW-1-px)) != 0)
		}
	}
}

// Canvas draws an animation on a Frame, one glyph per character cell. It
// implements anim.Canvas.
type Canvas struct{ F *Frame }

// Clear makes the frame white.
func (c Canvas) Clear() { c.F.Clear() }

// SetCell draws a character in a cell.
func (c Canvas) SetCell(x, y int, r rune) { c.F.DrawGlyph(x, y, r) }
