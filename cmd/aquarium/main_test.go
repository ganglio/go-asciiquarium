// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-asciiquarium/internal/app"
	"go-asciiquarium/internal/epd"
)

func TestSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tank.png")
	if err := takeSnapshot(app.New(1, 2), path, 2, true, false, 1); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != epd.Width*2 || b.Dy() != epd.Height*2 {
		t.Errorf("image is %v, want %dx%d", b, epd.Width*2, epd.Height*2)
	}

	// Black and white only, and with something drawn in it: the waves on top.
	black := 0
	for y := 0; y < epd.CellH*2; y++ {
		for x := 0; x < epd.Width*2; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != g || g != b || (r != 0 && r != 0xFFFF) {
				t.Fatalf("pixel (%d,%d) is not black or white", x, y)
			}
			if r == 0 {
				black++
			}
		}
	}
	if black == 0 {
		t.Error("nothing drawn in the top rows")
	}
}

func TestSnapshotMatchesTheFrame(t *testing.T) {
	var f epd.Frame
	f.Set(0, 0, true)
	f.Set(epd.Width-1, epd.Height-1, true)
	img := picture(&f, 3)
	for _, tt := range []struct {
		x, y  int
		white bool
	}{{0, 0, false}, {2, 2, false}, {3, 0, true}, {epd.Width*3 - 1, epd.Height*3 - 1, false}, {100, 100, true}} {
		_, _, _, _ = img.At(tt.x, tt.y).RGBA()
		if got := img.GrayAt(tt.x, tt.y).Y == 0xFF; got != tt.white {
			t.Errorf("pixel (%d,%d): white = %v, want %v", tt.x, tt.y, got, tt.white)
		}
	}
}

func TestSnapshotRejectsBadScale(t *testing.T) {
	if err := takeSnapshot(app.New(1, 2), filepath.Join(t.TempDir(), "x.png"), 0, false, false, 0); err == nil {
		t.Error("scale 0 was accepted")
	}
}

func TestPaceFromTheFlags(t *testing.T) {
	pace, err := paceFrom(time.Minute, 30*time.Second, "23:00-06:30", 5*time.Minute, 45*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := app.Pace{Idle: time.Minute, Tail: 30 * time.Second, Night: app.Night{From: 23 * time.Hour, To: 6*time.Hour + 30*time.Minute, Interval: 5 * time.Minute, Peek: 45 * time.Second}}
	if pace != want {
		t.Errorf("got %+v, want %+v", pace, want)
	}
	off, err := paceFrom(time.Minute, 30*time.Second, "off", 5*time.Minute, 45*time.Second)
	if err != nil || off.Night.Enabled() {
		t.Errorf("a night that is off: %+v, %v", off.Night, err)
	}
	for _, bad := range []struct {
		idle  time.Duration
		night string
		peek  time.Duration
	}{{0, "22:00-07:00", 0}, {time.Minute, "late", 0}, {time.Minute, "07:00-07:00", 0}, {time.Minute, "22:00-07:00", -time.Second}} {
		if _, err := paceFrom(bad.idle, time.Second, bad.night, time.Minute, bad.peek); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}

func TestSettingTheClock(t *testing.T) {
	now := time.Date(2026, time.November, 23, 14, 30, 15, 0, time.UTC)
	offset, err := clockOffset("21:58", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := now.Add(offset); got.Hour() != 21 || got.Minute() != 58 || got.Second() != 0 {
		t.Errorf("the clock says %v after being set to 21:58", got.Format("15:04:05"))
	}
	// With seconds too.
	offset, err = clockOffset("21:59:45", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := now.Add(offset); got.Hour() != 21 || got.Minute() != 59 || got.Second() != 45 {
		t.Errorf("the clock says %v after being set to 21:59:45", got.Format("15:04:05"))
	}
	if off, err := clockOffset("", now); err != nil || off != 0 {
		t.Errorf("not setting the clock: %v, %v", off, err)
	}
	for _, bad := range []string{"late", "25:00", "21:60", "9"} {
		if _, err := clockOffset(bad, now); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestTheKeysSkipTheClockToJustBeforeTheNightBeginsOrEnds(t *testing.T) {
	changed := 0
	term := &terminal{night: app.DefaultPace.Night, skipBy: skipTo, clockChanged: func() { changed++ }}
	term.skip(term.night.From)
	now, _ := term.Clock()
	if got := now.Sub(time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, now.Location())); got < -skipTo-time.Second || got > -skipTo+time.Second {
		t.Errorf("the clock is %v from ten in the evening, want about %v before", got, skipTo)
	}
	if changed != 1 {
		t.Errorf("the status line was told %d times", changed)
	}
}

func TestNightSnapshotIsInNegative(t *testing.T) {
	var day, night epd.Frame
	app.New(1, 2).Render(&day)

	tank := app.New(1, 2)
	path := filepath.Join(t.TempDir(), "night.png")
	if err := takeSnapshot(tank, path, 1, false, true, 5); err != nil {
		t.Fatal(err)
	}
	tank.Render(&night)
	if !tank.Asleep() {
		t.Error("the tank is not asleep")
	}
	black := func(f *epd.Frame) (n int) {
		for y := 0; y < epd.Height; y++ {
			for x := 0; x < epd.Width; x++ {
				if f.Black(x, y) {
					n++
				}
			}
		}
		return n
	}
	if d, n := black(&day), black(&night); d > epd.Width*epd.Height/10 || n < epd.Width*epd.Height*9/10 {
		t.Errorf("%d black pixels by day and %d at night: the night should be the negative", d, n)
	}
}

func TestInRealTimeTheKeysLandInTheNightAndInTheDay(t *testing.T) {
	night := app.DefaultPace.Night
	term := &terminal{night: night, skipBy: -liveSkip, clockChanged: func() {}}

	term.skip(night.From) // the key n
	if now, _ := term.Clock(); !night.At(now) {
		t.Errorf("after n the clock says %s, which is not night", now.Format("15:04:05"))
	}
	term.skip(night.To) // the key d
	if now, _ := term.Clock(); night.At(now) {
		t.Errorf("after d the clock says %s, which is not day", now.Format("15:04:05"))
	}
	// And at the badge's pace they are just before the change.
	term.skipBy = skipTo
	term.skip(night.From)
	if now, _ := term.Clock(); night.At(now) || !night.At(now.Add(skipTo)) {
		t.Errorf("at the badge's pace n puts the clock at %s: want just before night", now.Format("15:04:05"))
	}
}
