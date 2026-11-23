// SPDX-License-Identifier: GPL-2.0-or-later

// Aquarium runs the badge's aquarium on a computer, for debugging.
//
// It runs the same code as the firmware. By default it runs in real time: a
// step of the animation every tenth of a second, which is how the scene is made
// to be seen, so that you can check the fish, the bubbles, the feeding and the
// night at a glance. The badge cannot do that: its screen takes a second to
// refresh.
//
// With -badge it runs at the pace of the badge instead: the tank is a grid of
// 44x22 characters, shown once a minute when nothing is going on, and feeding
// the fish makes the pictures come back to back until the fish has eaten
// everything and for half a minute more. The terminal shows the characters;
// what the badge's screen would do is counted in the status line, and the
// refresh time of the e-ink is simulated (-refresh). To see that without
// waiting a minute: make run-badge ARGS="-idle 3s -tail 5s".
//
// At night, between two times of day (-night 22:00-07:00), the tank goes to
// sleep: no fish, the picture in negative, and bubbles that rise from the
// algae. The badge reads the time from its real-time clock; here it is the
// clock of this computer, which you can set (-time 21:58) or skip ahead (keys n
// and d). The button at night is a peek at the day (-night-peek 30s) with no
// feeding; on the badge it is the picture and then the night again.
//
// To see it quickly: make run ARGS="-time 21:59:50", or press n.
//
// Keys: f feed the fish, n the clock jumps to just after night begins, d to
// just after it ends (with -badge: to 5 s before, for the next picture to see
// it), r start over with a new tank, q quit.
//
// With -snapshot the tank is drawn to a PNG file instead, exactly as it would
// be on the badge's screen, without a terminal.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"go-asciiquarium/internal/app"
	"go-asciiquarium/internal/epd"
)

const (
	// defaultRefresh is how long the badge's screen takes to refresh.
	defaultRefresh = time.Second

	// defaultScale is how many pixels of the PNG a pixel of the screen gets.
	defaultScale = 3

	// seedStride sets the second seed apart from the first: it is the golden
	// ratio as a 64-bit number, which is the usual way to spread seeds.
	seedStride uint64 = 0x9E3779B97F4A7C15

	// statusRow is where the status line goes: one row of space under the tank.
	statusRow = epd.Rows + 1

	// skipTo is how long before night begins, or ends, the keys n and d put the
	// clock when the program is at the badge's pace: it takes a picture or two
	// to notice, and the clock is then right at the change.
	skipTo = 5 * time.Second

	// liveSkip is how long after night begins, or ends, they put it in real
	// time, where the change is to be seen as soon as the key is pressed.
	liveSkip = time.Second

	// How a time of day is written in -time: with or without the seconds.
	clockFormat        = "15:04"
	clockFormatSeconds = "15:04:05"
)

func main() {
	seed := flag.Uint64("seed", 0, "seed for the tank; the same seed gives the same tank (0: random)")
	badge := flag.Bool("badge", false, "run at the pace of the badge, with its e-ink screen simulated, instead of in real time")
	refresh := flag.Duration("refresh", defaultRefresh, "with -badge: how long the simulated e-ink screen takes to refresh")
	idle := flag.Duration("idle", app.DefaultPace.Idle, "with -badge: time between pictures when nothing is going on")
	tail := flag.Duration("tail", app.DefaultPace.Tail, "with -badge: how long pictures keep coming back to back after the fish has eaten the last pellet")
	night := flag.String("night", app.DefaultPace.Night.String(), `the night: two times of day, like 22:00-07:00, or "off"`)
	nightInterval := flag.Duration("night-interval", app.DefaultPace.Night.Interval, "with -badge: time between pictures at night")
	nightPeek := flag.Duration("night-peek", app.DefaultPace.Night.Peek, "how long the button shows the day at night, with no feeding (0: the button does nothing at night)")
	clock := flag.String("time", "", "the time of day on the clock when the program starts, like 21:58 or 21:58:30 (default: the real one)")
	snapshot := flag.String("snapshot", "", "write the tank to this PNG file and exit, instead of running")
	feed := flag.Bool("feed", false, "with -snapshot: feed the fish first")
	asleep := flag.Bool("asleep", false, "with -snapshot: it is night")
	steps := flag.Int("steps", 0, "with -snapshot: how many updates to run before taking the picture")
	scale := flag.Int("scale", defaultScale, "with -snapshot: size of a screen pixel in the PNG")
	flag.Parse()

	if *seed == 0 {
		*seed = rand.Uint64()
	}
	tank := app.New(*seed, *seed^seedStride)

	pace, err := paceFrom(*idle, *tail, *night, *nightInterval, *nightPeek)
	offset, clockErr := clockOffset(*clock, time.Now())
	switch {
	case err != nil:
	case clockErr != nil:
		err = clockErr
	case *snapshot != "":
		err = takeSnapshot(tank, *snapshot, *scale, *feed, *asleep, *steps)
	case *badge:
		err = run(tank, pace, *seed, *refresh, offset)
	default:
		err = runLive(tank, pace.Night, *seed, offset)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "aquarium:", err)
		os.Exit(1)
	}
}

// paceFrom makes a pace out of what the flags say.
func paceFrom(idle, tail time.Duration, night string, nightInterval, nightPeek time.Duration) (app.Pace, error) {
	pace := app.Pace{Idle: idle, Tail: tail}
	if night != "off" {
		from, to, err := app.ParseNight(night)
		if err != nil {
			return pace, err
		}
		pace.Night = app.Night{From: from, To: to, Interval: nightInterval, Peek: nightPeek}
	}
	return pace, pace.Validate()
}

// clockOffset is how far the clock of the program is from the real one: none
// by default, or whatever makes it say the given time of day now.
func clockOffset(setTo string, now time.Time) (time.Duration, error) {
	if setTo == "" {
		return 0, nil
	}
	t, err := time.Parse(clockFormatSeconds, setTo)
	if err != nil {
		t, err = time.Parse(clockFormat, setTo)
	}
	if err != nil {
		return 0, fmt.Errorf("-time is a time of day like 21:58 or 21:58:30, not %q", setTo)
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	atTime := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
	return midnight.Add(atTime).Sub(now), nil
}

// what the status line says about each kind of picture.
var kindLabel = map[app.Kind]string{
	app.Burst:   "feeding",
	app.Settle:  "settled",
	app.Ambient: "idle",
}

// run shows the tank in a terminal and runs it at the pace of the badge.
func run(tank *app.App, pace app.Pace, seed uint64, refresh time.Duration, clockOffset time.Duration) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()

	sim := &epd.Sim{RefreshTime: refresh}
	display := epd.New(sim, epd.Config{})
	var frame epd.Frame
	last := app.Settle
	env := &terminal{offset: clockOffset, night: pace.Night, skipBy: skipTo}

	status := func() {
		s := display.Stats()
		label := kindLabel[last]
		if tank.Asleep() {
			label = "night"
			if last == app.Burst {
				label = "night, feeding"
			}
		}
		now, _ := env.Clock()
		line := fmt.Sprintf("%s | pictures %d, skipped %d, cleans %d, refreshes %d | clock %s, night %v | idle %v, tail %v | seed %d | f feed  n night  d day  r new tank  q quit",
			label, s.Pictures, s.Skipped, s.Cleans, sim.Refreshes, now.Format("15:04:05"), nightLabel(pace.Night), pace.Idle, pace.Tail, seed)
		showStatus(screen, line)
	}

	redraw := func() {
		tank.Draw(tcellCanvas{screen: screen, negative: tank.Asleep()})
		status()
	}

	// present shows the tank on the terminal and then on the simulated panel,
	// which takes its time to refresh.
	present := func(kind app.Kind) {
		last = kind
		redraw()
		tank.Render(&frame)
		display.Show(&frame, kind == app.Settle)
		status() // the counts have changed
	}

	env.events = pollEvents(screen)
	env.resized = func() {
		screen.Sync()
		redraw()
	}
	env.rebuilt = func() {
		tank.Rebuild()
		present(app.Settle)
	}
	env.clockChanged = status
	tank.Run(env, pace, present)
	return nil
}

// showStatus writes the status line under the tank, and shows what is on the
// screen.
func showStatus(screen tcell.Screen, line string) {
	width, _ := screen.Size()
	for x := range width {
		r := ' '
		if x < len(line) {
			r = rune(line[x])
		}
		screen.SetContent(x, statusRow, r, nil, tcell.StyleDefault)
	}
	screen.Show()
}

// pollEvents passes what happens in the terminal on to a channel.
func pollEvents(screen tcell.Screen) <-chan tcell.Event {
	events := make(chan tcell.Event)
	go func() {
		for ev := screen.PollEvent(); ev != nil; ev = screen.PollEvent() {
			events <- ev
		}
	}()
	return events
}

// terminal is the outside world of the tank when it runs in a terminal: the
// clock, and the keyboard standing in for the button.
type terminal struct {
	events       <-chan tcell.Event
	resized      func() // the terminal changed size
	rebuilt      func() // the user asked for a new tank
	clockChanged func() // the user skipped the clock ahead

	offset time.Duration // the clock is this far from the real one
	night  app.Night
	skipBy time.Duration // n and d put the clock this long before the change; negative is after
}

func (t *terminal) Now() time.Time { return time.Now() }

// Clock is the clock of this computer, as far off as it was set. There is
// always a way to tell.
func (t *terminal) Clock() (time.Time, bool) { return time.Now().Add(t.offset), true }

// skip sets the clock to skipBy before a time of day, which goes on from there.
func (t *terminal) skip(timeOfDay time.Duration) {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	t.offset = midnight.Add(timeOfDay - t.skipBy).Sub(now)
	t.clockChanged()
}

// nightLabel says when the night is, for the status line.
func nightLabel(n app.Night) string {
	if !n.Enabled() {
		return "off"
	}
	return n.String()
}

// Wait sleeps for up to d, or until f is pressed, which feeds the fish, or q,
// which quits. It looks at the keyboard without waiting if d is zero.
func (t *terminal) Wait(d time.Duration) app.Event {
	var timeout <-chan time.Time
	if d > 0 {
		timer := time.NewTimer(d)
		defer timer.Stop()
		timeout = timer.C
	}
	for {
		if d == 0 {
			select {
			case ev := <-t.events:
				if event, ended := t.handle(ev); ended {
					return event
				}
			default:
				return app.Timeout
			}
			continue
		}
		select {
		case ev := <-t.events:
			if event, ended := t.handle(ev); ended {
				return event
			}
		case <-timeout:
			return app.Timeout
		}
	}
}

// handle reacts to something that happened in the terminal. If ended is set
// Wait is over and event says why.
func (t *terminal) handle(ev tcell.Event) (event app.Event, ended bool) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		t.resized()
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyCtrlC {
			return app.Quit, true
		}
		if ev.Key() != tcell.KeyRune {
			break
		}
		switch unicode.ToLower(ev.Rune()) {
		case 'q':
			return app.Quit, true
		case 'f':
			return app.Pressed, true
		case 'r':
			t.rebuilt()
		case 'n':
			if t.night.Enabled() {
				t.skip(t.night.From)
			}
		case 'd':
			if t.night.Enabled() {
				t.skip(t.night.To)
			}
		}
	}
	return app.Timeout, false
}

// tcellCanvas draws the tank's character grid on the terminal, in negative at
// night as the badge's screen does.
type tcellCanvas struct {
	screen   tcell.Screen
	negative bool
}

func (c tcellCanvas) style() tcell.Style { return tcell.StyleDefault.Reverse(c.negative) }

func (c tcellCanvas) Clear() {
	for y := 0; y < epd.Rows; y++ {
		for x := 0; x < epd.Cols; x++ {
			c.screen.SetContent(x, y, ' ', nil, c.style())
		}
	}
}

func (c tcellCanvas) SetCell(x, y int, r rune) {
	c.screen.SetContent(x, y, r, nil, c.style())
}
