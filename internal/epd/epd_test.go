// SPDX-License-Identifier: GPL-2.0-or-later

package epd

import (
	"bytes"
	"slices"
	"testing"

	"go-asciiquarium/internal/flip"
)

// ramSize is how many bytes the controller's RAM holds for one picture: one
// bit for each of the 264 x 176 pixels, which is 5808 bytes.
const ramSize = Width * Height / bitsPerByte

func TestFrameLayout(t *testing.T) {
	var f Frame
	// Pixel (0,0) is the top bit of the first byte; the second column starts a
	// column of bytes further; the last pixel is the lowest bit of the last byte.
	const lowestBit = 1
	for _, tt := range []struct {
		x, y  int
		index int
		bit   byte
	}{
		{0, 0, 0, topBit},
		{0, bitsPerByte - 1, 0, lowestBit},
		{0, bitsPerByte, 1, topBit},
		{1, 0, bytesPerColumn, topBit},
		{Width - 1, Height - 1, Width*bytesPerColumn - 1, lowestBit},
	} {
		f.Clear()
		f.Set(tt.x, tt.y, true)
		n := f.Native()
		if len(n) != ramSize {
			t.Fatalf("native size = %d, want %d", len(n), ramSize)
		}
		for i, b := range n {
			want := byte(0)
			if i == tt.index {
				want = tt.bit
			}
			if b != want {
				t.Fatalf("Set(%d,%d): byte %d = %#x, want %#x", tt.x, tt.y, i, b, want)
			}
		}
		if !f.Black(tt.x, tt.y) {
			t.Errorf("Black(%d,%d) = false", tt.x, tt.y)
		}
		f.Set(tt.x, tt.y, false)
		if f.Black(tt.x, tt.y) {
			t.Errorf("pixel (%d,%d) did not turn white", tt.x, tt.y)
		}
	}
}

func TestFrameBounds(t *testing.T) {
	var f Frame
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {Width, 0}, {0, Height}, {1000, 1000}} {
		f.Set(p[0], p[1], true) // must not panic or touch anything
		if f.Black(p[0], p[1]) {
			t.Errorf("pixel %v is black", p)
		}
	}
	if !f.Equal(&Frame{}) {
		t.Error("out-of-range Set changed the frame")
	}
}

func TestFrameFillClearEqual(t *testing.T) {
	var a, b Frame
	if !a.Equal(&b) {
		t.Fatal("two white frames differ")
	}
	a.Fill()
	if a.Equal(&b) || !a.Black(0, 0) || !a.Black(Width-1, Height-1) {
		t.Error("Fill did not make the frame black")
	}
	a.Clear()
	if !a.Equal(&b) {
		t.Error("Clear did not make the frame white")
	}
}

func TestGeometry(t *testing.T) {
	if Cols != 44 || Rows != 22 || Cols*CellW != Width || Rows*CellH != Height {
		t.Errorf("grid is %dx%d cells", Cols, Rows)
	}
}

// pixels returns the cell at (col,row) as rows of '#' and '.'.
func pixels(f *Frame, col, row int) [CellH]string {
	var out [CellH]string
	for y := 0; y < CellH; y++ {
		line := make([]byte, CellW)
		for x := 0; x < CellW; x++ {
			line[x] = '.'
			if f.Black(col*CellW+x, row*CellH+y) {
				line[x] = '#'
			}
		}
		out[y] = string(line)
	}
	return out
}

func TestGlyphsAreDrawnAsDesigned(t *testing.T) {
	var f Frame
	for r, art := range glyphArt {
		f.DrawGlyph(3, 5, r)
		if got := pixels(&f, 3, 5); got != art {
			t.Errorf("glyph %q drawn as %q, want %q", r, got, art)
		}
		// Nothing leaks into the neighbouring cells.
		for _, c := range [][2]int{{2, 5}, {4, 5}, {3, 4}, {3, 6}} {
			if got := pixels(&f, c[0], c[1]); got != glyphArt[' '] {
				t.Errorf("glyph %q spilled into cell %v", r, c)
			}
		}
	}
}

func TestDrawingAGlyphReplacesTheCell(t *testing.T) {
	var f Frame
	f.DrawGlyph(0, 0, '*')
	f.DrawGlyph(0, 0, ' ')
	if !f.Equal(&Frame{}) {
		t.Error("a space left the previous glyph behind")
	}
}

func TestEveryGlyphIsWellFormed(t *testing.T) {
	// compileGlyphs panics on a malformed glyph; this checks it ran.
	if want := len(glyphArt) + len(flip.Pairs); len(glyphs) != want {
		t.Errorf("%d glyphs compiled, %d drawn and %d made", len(glyphs), len(glyphArt), len(flip.Pairs))
	}
	for r := range glyphArt {
		if !HasGlyph(r) {
			t.Errorf("no glyph for %q", r)
		}
	}
	if HasGlyph('Z') {
		t.Error("the font should only have what the aquarium uses")
	}
}

func TestMissingGlyphIsVisible(t *testing.T) {
	var f Frame
	f.DrawGlyph(1, 1, 'Z')
	if got := pixels(&f, 1, 1); got != missingGlyph || got == glyphArt[' '] {
		t.Errorf("unknown character drawn as %q", got)
	}
}

func TestWaveJoinsUpWithItself(t *testing.T) {
	// A row of '~' is one continuous wave: the line leaves a cell where the
	// next one picks it up.
	var f Frame
	f.DrawGlyph(0, 0, '~')
	f.DrawGlyph(1, 0, '~')
	if !f.Black(CellW-1, 5) || !f.Black(CellW, 4) {
		t.Error("the wave breaks between two cells")
	}
}

func TestCanvasDrawsCells(t *testing.T) {
	var f Frame
	c := Canvas{&f}
	c.SetCell(2, 3, '*')
	if got := pixels(&f, 2, 3); got != glyphArt['*'] {
		t.Errorf("cell drawn as %q", got)
	}
	c.SetCell(Cols, 0, '*') // outside: ignored
	c.SetCell(0, Rows, '*')
	c.Clear()
	if !f.Equal(&Frame{}) {
		t.Error("Clear left something behind")
	}
}

// event is something the display did on the transport.
type event struct {
	kind string // reset, cmd, data, wait
	cmd  byte
	data []byte
}

// recorder is a Transport that remembers everything.
type recorder struct{ events []event }

func (r *recorder) Reset()    { r.events = append(r.events, event{kind: "reset"}) }
func (r *recorder) WaitIdle() { r.events = append(r.events, event{kind: "wait"}) }
func (r *recorder) Command(cmd byte, params ...byte) {
	r.events = append(r.events, event{"cmd", cmd, slices.Clone(params)})
}
func (r *recorder) Data(data ...byte) {
	r.events = append(r.events, event{kind: "data", data: slices.Clone(data)})
}

// commands lists the command bytes sent, in order.
func (r *recorder) commands() []byte {
	var out []byte
	for _, e := range r.events {
		if e.kind == "cmd" {
			out = append(out, e.cmd)
		}
	}
	return out
}

// count says how many times a command was sent.
func (r *recorder) count(cmd byte) int {
	n := 0
	for _, c := range r.commands() {
		if c == cmd {
			n++
		}
	}
	return n
}

// ramWrites returns the pictures written to a RAM plane, in order.
func (r *recorder) ramWrites(plane byte) [][]byte {
	var out [][]byte
	for i, e := range r.events {
		if e.kind == "cmd" && e.cmd == plane {
			out = append(out, r.events[i+1].data)
		}
	}
	return out
}

func (r *recorder) reset() { r.events = nil }

func pictureWith(black ...[2]int) *Frame {
	f := new(Frame)
	for _, p := range black {
		f.Set(p[0], p[1], true)
	}
	return f
}

func TestFirstPictureCleansTheScreenFirst(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{})
	pic := pictureWith([2]int{0, 0})
	if !d.Show(pic, false) {
		t.Fatal("the first picture was not shown")
	}
	// Black, white, then the picture: three refreshes, and a single setup.
	if got := r.count(regActivate); got != 3 {
		t.Errorf("%d refreshes, want 3", got)
	}
	if r.count(regSoftReset) != 1 || r.count(regDriverOutput) != 1 {
		t.Error("the controller should be set up exactly once")
	}
	bw := r.ramWrites(regRAMBW)
	if len(bw) != 3 {
		t.Fatalf("%d RAM writes, want 3", len(bw))
	}
	if !bytes.Equal(bw[0], bytes.Repeat([]byte{byteMax}, ramSize)) {
		t.Error("the first refresh should be all black")
	}
	if !bytes.Equal(bw[1], make([]byte, ramSize)) {
		t.Error("the second refresh should be all white")
	}
	if !bytes.Equal(bw[2], pic.Native()) {
		t.Error("the third refresh should be the picture")
	}
	if s := d.Stats(); s.Pictures != 1 || s.Cleans != 1 || s.Skipped != 0 {
		t.Errorf("stats = %+v", s)
	}
}

func TestSamePictureIsNotShownAgain(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{})
	pic := pictureWith([2]int{5, 5})
	d.Show(pic, false)
	r.reset()

	same := pictureWith([2]int{5, 5})
	if d.Show(same, false) {
		t.Error("an unchanged picture touched the screen")
	}
	if len(r.events) != 0 {
		t.Errorf("%d events for an unchanged picture", len(r.events))
	}
	if d.Stats().Skipped != 1 {
		t.Errorf("stats = %+v", d.Stats())
	}

	if !d.Show(pictureWith([2]int{6, 5}), false) {
		t.Error("a changed picture was not shown")
	}
	if r.count(regActivate) != 1 {
		t.Errorf("%d refreshes for one changed picture", r.count(regActivate))
	}
}

func TestPolarityAndBothPlanes(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{})
	pic := pictureWith([2]int{0, 0}, [2]int{263, 175})
	d.Show(pic, false)

	// A black pixel is a 1 in both RAM planes, as in Pimoroni's firmware.
	for _, plane := range []byte{regRAMBW, regRAMRed} {
		w := r.ramWrites(plane)
		last := w[len(w)-1]
		if last[0] != 0x80 || last[ramSize-1] != 0x01 {
			t.Errorf("plane %#x: first byte %#x, last byte %#x", plane, last[0], last[ramSize-1])
		}
	}
	red, bw := r.ramWrites(regRAMRed), r.ramWrites(regRAMBW)
	for i := range red {
		if !bytes.Equal(red[i], bw[i]) {
			t.Errorf("update %d: the two RAM planes differ", i)
		}
	}
}

func TestSetupAndUpdateSequence(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{CleanEvery: 100})
	d.Show(pictureWith([2]int{1, 1}), false)
	r.reset()
	d.Show(pictureWith([2]int{2, 2}), false) // one plain update

	var seq []string
	for _, e := range r.events {
		switch e.kind {
		case "cmd":
			seq = append(seq, string([]byte{"0123456789abcdef"[e.cmd>>4], "0123456789abcdef"[e.cmd&15]}))
		case "data":
			seq = append(seq, "data")
		case "wait":
			seq = append(seq, "wait")
		}
	}
	got := join(seq)
	want := join([]string{
		"wait",
		// the waveform
		"32", "data", "data", "data", "data",
		"3f", "03", "04", "2c", "wait",
		// the picture, into both planes
		"4e", "4f", "26", "data",
		"4e", "4f", "24", "data",
		// refresh
		"0c", "22", "wait", "20", "wait",
	})
	if got != want {
		t.Errorf("update sequence\n got %s\nwant %s", got, want)
	}
}

func join(s []string) string {
	out := ""
	for _, x := range s {
		out += x + " "
	}
	return out
}

func TestWaveformAndRegisterValues(t *testing.T) {
	if len(lut) != 153 {
		t.Fatalf("waveform is %d bytes, want 153", len(lut))
	}
	if !slices.Equal(lut[:2], []byte{0x40, 0x68}) || !slices.Equal(lut[144:], []byte{0x44, 0x42, 0x22, 0x22, 0x23, 0x32, 0, 0, 0}) {
		t.Error("waveform does not start/end as in the firmware")
	}

	r := &recorder{}
	New(r, Config{}).Show(new(Frame), false)
	find := func(cmd byte) []byte {
		for _, e := range r.events {
			if e.kind == "cmd" && e.cmd == cmd {
				return e.data
			}
		}
		t.Fatalf("command %#x never sent", cmd)
		return nil
	}
	for cmd, want := range map[byte][]byte{
		regDriverOutput: {0x07, 0x01, 0x00},
		regEntryMode:    {0x01},
		regRAMXRange:    {0x00, 0x15},
		regRAMYRange:    {0x07, 0x01, 0x00, 0x00},
		regRAMYCounter:  {0x07, 0x01},
		regUpdateCtrl2:  {0xC7},
		regEndOption:    {0x22},
		regGateVoltage:  {0x17},
		regSourceVolt:   {0x41, 0xAE, 0x32},
		regVCOM:         {0x28},
	} {
		if got := find(cmd); !slices.Equal(got, want) {
			t.Errorf("command %#x sent with % x, want % x", cmd, got, want)
		}
	}

	// The waveform goes out in one piece of 153 bytes however it is chunked.
	var all []byte
	collecting := false
	for _, e := range r.events {
		if e.kind == "cmd" && e.cmd == regLUT {
			collecting = true
			all = append(all, e.data...)
			continue
		}
		if collecting {
			if e.kind != "data" {
				break
			}
			all = append(all, e.data...)
		}
	}
	if !bytes.Equal(all, lut) {
		t.Errorf("waveform sent as %d bytes, differs from the table", len(all))
	}
}

func TestCleaningEveryNPictures(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{CleanEvery: 3, SettleMin: 100})
	d.Show(pictureWith([2]int{0, 0}), false) // clean + picture
	cleansAfter := func() int { return d.Stats().Cleans }
	if cleansAfter() != 1 {
		t.Fatalf("cleans = %d", cleansAfter())
	}
	var seen []int
	for i := 1; i <= 7; i++ {
		d.Show(pictureWith([2]int{i, 0}), false)
		seen = append(seen, cleansAfter())
	}
	// Pictures 2 and 3 (debt 2, 3), then a cleaning before picture 4, and so on.
	want := []int{1, 1, 2, 2, 2, 3, 3}
	if !slices.Equal(seen, want) {
		t.Errorf("cleans after each picture = %v, want %v", seen, want)
	}
	if d.Stats().Pictures != 8 {
		t.Errorf("pictures = %d", d.Stats().Pictures)
	}
}

func TestSettleCleansAtTheEndOfABurst(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{CleanEvery: 100, SettleMin: 3})
	d.Show(pictureWith([2]int{0, 0}), true) // boot: clean, then 1 picture (debt 1)

	d.Show(pictureWith([2]int{1, 0}), false) // debt 2
	cleans := d.Stats().Cleans
	// Not final and below CleanEvery: no cleaning.
	d.Show(pictureWith([2]int{2, 0}), false) // debt 3
	if d.Stats().Cleans != cleans {
		t.Error("cleaned in the middle of a burst")
	}

	// The same picture, but it is the last one and the debt is at SettleMin:
	// the screen is cleaned and the picture drawn again, though it didn't change.
	r.reset()
	if !d.Show(pictureWith([2]int{2, 0}), true) {
		t.Fatal("the final picture was skipped although the screen needed cleaning")
	}
	if d.Stats().Cleans != cleans+1 || r.count(regActivate) != 3 {
		t.Errorf("cleans=%d refreshes=%d", d.Stats().Cleans, r.count(regActivate))
	}

	// With the debt paid, a final unchanged picture does nothing.
	r.reset()
	if d.Show(pictureWith([2]int{2, 0}), true) {
		t.Error("touched the screen for an unchanged final picture with no debt")
	}
}

func TestWaitsForTheScreenBeforeEveryUpdate(t *testing.T) {
	r := &recorder{}
	d := New(r, Config{CleanEvery: 100})
	d.Show(pictureWith([2]int{0, 0}), false)
	r.reset()
	d.Show(pictureWith([2]int{1, 0}), false)
	if r.events[0].kind != "wait" {
		t.Errorf("update starts with %v, want a wait for the previous refresh", r.events[0])
	}
	last := r.events[len(r.events)-1]
	if last.kind != "wait" {
		t.Errorf("Show returned before the refresh was over (last event %v)", last)
	}
}

func TestSimTransport(t *testing.T) {
	s := &Sim{}
	d := New(s, Config{})
	d.Show(pictureWith([2]int{0, 0}), false)
	d.Show(pictureWith([2]int{1, 0}), false)
	if s.Refreshes != 4 { // black, white, picture 1, picture 2
		t.Errorf("%d refreshes, want 4", s.Refreshes)
	}
}

// shape is a glyph cut down to the smallest box that holds its black pixels,
// so that glyphs can be compared wherever they sit in the cell.
func shape(art [CellH]string) []string {
	minX, maxX, minY, maxY := CellW, -1, CellH, -1
	for y, row := range art {
		for x, c := range row {
			if c == '#' {
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	var out []string
	for y := minY; y <= maxY; y++ {
		out = append(out, art[y][minX:maxX+1])
	}
	return out
}

func flipped(rows []string) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		r := []byte(row)
		slices.Reverse(r)
		out[i] = string(r)
	}
	return out
}

// columns says where a glyph's black pixels start and end, left to right.
func columns(art [CellH]string) (first, last int) {
	first, last = CellW, -1
	for _, row := range art {
		for x := range row {
			if row[x] == '#' {
				first, last = min(first, x), max(last, x)
			}
		}
	}
	return first, last
}

func TestMirroredCharactersAreMadeNotDrawn(t *testing.T) {
	for _, pair := range flip.Pairs {
		drawn, made := pair[0], pair[1]
		if _, ok := glyphArt[drawn]; !ok {
			t.Errorf("%q should be drawn in the font", drawn)
		}
		if _, ok := glyphArt[made]; ok {
			t.Errorf("%q should be made from %q, not drawn: that is writing it down twice", made, drawn)
		}
		if !HasGlyph(drawn) || !HasGlyph(made) {
			t.Errorf("no glyph for %q or %q", drawn, made)
			continue
		}

		// What is drawn on the frame is the mirror image of the other one...
		var a, b Frame
		a.DrawGlyph(0, 0, drawn)
		b.DrawGlyph(0, 0, made)
		got, want := pixels(&b, 0, 0), mirrorGlyph(glyphArt[drawn])
		if got != want {
			t.Errorf("%q is drawn as\n%s\nwant\n%s", made, join(got[:]), join(want[:]))
		}
		if !slices.Equal(flipped(shape(pixels(&a, 0, 0))), shape(pixels(&b, 0, 0))) {
			t.Errorf("%q is not the mirror image of %q", made, drawn)
		}
		if a.Equal(&b) {
			t.Errorf("%q and %q look the same", drawn, made)
		}

		// ... and keeps its place in the cell, so the spacing stays right.
		f1, l1 := columns(pixels(&a, 0, 0))
		f2, l2 := columns(pixels(&b, 0, 0))
		if f1 != f2 || l1 != l2 {
			t.Errorf("%q spans columns %d-%d, %q spans %d-%d", drawn, f1, l1, made, f2, l2)
		}
	}
}

func TestMirrorGlyph(t *testing.T) {
	// Flipped around the glyph's own box, not around the cell: it stays put.
	// The glyph is at the edge of the cell, where the two would not agree.
	art := [CellH]string{
		"......",
		"#.....",
		"##....",
		"......",
		"......",
		"......",
		"......",
		"......",
	}
	want := [CellH]string{
		"......",
		".#....",
		"##....",
		"......",
		"......",
		"......",
		"......",
		"......",
	}
	if got := mirrorGlyph(art); got != want {
		t.Errorf("got\n%s\nwant\n%s", join(got[:]), join(want[:]))
	}
	if got := mirrorGlyph(want); got != art {
		t.Error("mirroring twice does not give the glyph back")
	}
	// A blank glyph has nothing to flip.
	if got := mirrorGlyph(glyphArt[' ']); got != glyphArt[' '] {
		t.Error("the blank glyph changed")
	}
}

func TestInvert(t *testing.T) {
	var f, white Frame
	f.Set(3, 4, true)
	f.Set(Width-1, Height-1, true)
	f.Invert()
	if f.Black(3, 4) || f.Black(Width-1, Height-1) || !f.Black(0, 0) || !f.Black(100, 100) {
		t.Error("black and white were not swapped")
	}
	f.Invert()
	if !f.Black(3, 4) || !f.Black(Width-1, Height-1) || f.Black(0, 0) {
		t.Error("inverting twice did not give the frame back")
	}
	white.Invert()
	var black Frame
	black.Fill()
	if !white.Equal(&black) {
		t.Error("a white frame inverted is not a black one")
	}
}
