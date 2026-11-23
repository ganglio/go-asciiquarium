// SPDX-License-Identifier: GPL-2.0-or-later

// Package app is what runs on the badge and, for debugging, on a computer:
// the tank and the pace it lives at. It knows nothing about the hardware.
//
// An e-ink screen is slow and each refresh costs battery, so the tank mostly
// sleeps. When nothing is going on there is one picture every so often (a
// minute), showing the tank as it is by then. Feeding the fish wakes everything
// up: pictures come back to back, as fast as the screen refreshes, while there
// is food in the tank and for a while after, and then it falls back to the slow
// pace. At night, between two times of the day, the tank goes to sleep: the
// fish is gone, the picture is the negative of the day's, and there is one every
// ten minutes, of the algae swaying and a few bubbles slowly rising.
package app

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"go-asciiquarium/internal/anim"
	"go-asciiquarium/internal/aquarium"
	"go-asciiquarium/internal/epd"
)

const (
	// StepsPerUpdate is how many animation steps pass between two of the
	// pictures that come back to back. The scene is made for steps of a tenth
	// of a second, but a refresh of the screen takes about a second, so each
	// picture moves things on by several steps: the fish swims a few cells, a
	// bubble rises a few rows.
	StepsPerUpdate = 6

	// StepDuration is how much time one animation step stands for.
	StepDuration = 100 * time.Millisecond

	// MaxBurstUpdates bounds a run of pictures back to back, so that the screen
	// can't be refreshed forever if something is wrong.
	MaxBurstUpdates = 400

	// defaultIdle is the time between two pictures when nothing is going on.
	defaultIdle = time.Minute

	// defaultTail is how long the pictures keep coming back to back after the
	// fish has eaten the last pellet.
	defaultTail = 30 * time.Second

	// The night: from ten in the evening to seven in the morning, with a
	// picture of the sleeping tank every ten minutes.
	defaultNightFrom     = 22 * time.Hour
	defaultNightTo       = 7 * time.Hour
	defaultNightInterval = 10 * time.Minute

	// defaultNightPeek is how long the tank stays awake when the button is
	// pressed at night.
	defaultNightPeek = 30 * time.Second

	day = 24 * time.Hour
)

// Pace is how often the tank is shown.
type Pace struct {
	// Idle is the time between two pictures when nothing is going on. In that
	// time the tank lives as much as it would in the real world.
	Idle time.Duration
	// Tail is how long the pictures keep coming back to back after the fish
	// has eaten the last pellet.
	Tail time.Duration
	// Night is when the tank goes to sleep. The zero value means never.
	Night Night
}

// Night says when the tank sleeps: between two times of the day, every day.
type Night struct {
	// From is the time of day the night begins and To the time it ends, each
	// as the time since midnight. From is usually the later of the two: the
	// night goes through midnight.
	From, To time.Duration
	// Interval is how often a picture of the sleeping tank is shown. Zero
	// means there is no night.
	Interval time.Duration
	// Peek is how long the tank is shown as it is by day, with no feeding, when
	// the button is pressed at night, before it goes back to sleep. Zero means
	// the button does nothing at night.
	Peek time.Duration
}

// Enabled reports whether the tank ever goes to sleep.
func (n Night) Enabled() bool { return n.Interval > 0 && n.From != n.To }

// timeOfDay is how long after midnight a time on the wall is.
func timeOfDay(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second + time.Duration(t.Nanosecond())
}

// At reports whether it is night at a time on the wall.
func (n Night) At(t time.Time) bool {
	if !n.Enabled() {
		return false
	}
	now := timeOfDay(t)
	if n.From < n.To {
		return now >= n.From && now < n.To
	}
	return now >= n.From || now < n.To // through midnight
}

// untilEnd is how much is left of the night that goes on at a time.
func (n Night) untilEnd(t time.Time) time.Duration {
	left := n.To - timeOfDay(t)
	if left <= 0 {
		left += day
	}
	return left
}

// state says whether it is night by the clock, and if so how much of it is
// left. If the clock can't be read it never is night.
func (n Night) state(env Env) (night bool, left time.Duration) {
	if !n.Enabled() {
		return false, 0
	}
	t, ok := env.Clock()
	if !ok || !n.At(t) {
		return false, 0
	}
	return true, n.untilEnd(t)
}

// timeFormat is how a time of day is written: 22:00.
const timeFormat = "15:04"

// ParseNight reads the two times of a night, written like "22:00-07:00".
func ParseNight(s string) (from, to time.Duration, err error) {
	start, end, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("a night is two times of day, like 22:00-07:00, not %q", s)
	}
	if from, err = parseTimeOfDay(start); err != nil {
		return 0, 0, err
	}
	if to, err = parseTimeOfDay(end); err != nil {
		return 0, 0, err
	}
	return from, to, nil
}

func parseTimeOfDay(s string) (time.Duration, error) {
	t, err := time.Parse(timeFormat, strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("a time of day is written like 07:30, not %q", s)
	}
	return timeOfDay(t), nil
}

// String writes the night as ParseNight reads it.
func (n Night) String() string {
	at := func(d time.Duration) string { return time.Time{}.Add(d).Format(timeFormat) }
	return at(n.From) + "-" + at(n.To)
}

// Validate says what is wrong with a pace, if anything.
func (p Pace) Validate() error {
	switch {
	case p.Idle <= 0:
		return fmt.Errorf("the time between pictures must be more than zero, not %v", p.Idle)
	case p.Tail < 0:
		return fmt.Errorf("the tail must not be negative, not %v", p.Tail)
	case p.Night.Interval < 0:
		return fmt.Errorf("the time between pictures at night must not be negative, not %v", p.Night.Interval)
	case p.Night.Peek < 0:
		return fmt.Errorf("the peek at night must not be negative, not %v", p.Night.Peek)
	case p.Night.From < 0 || p.Night.From >= day || p.Night.To < 0 || p.Night.To >= day:
		return fmt.Errorf("the night must begin and end within a day, not %v to %v", p.Night.From, p.Night.To)
	case p.Night.Interval > 0 && p.Night.From == p.Night.To:
		return fmt.Errorf("the night must begin and end at different times, not both at %v", p.Night.From)
	}
	return nil
}

// Overrides are settings, as text, that replace parts of a pace: durations
// like "2m" and a night like "23:00-06:30". What is empty is left alone.
type Overrides struct {
	Idle, Tail, Night, NightInterval, NightPeek string
}

// With is the pace with the overrides applied.
func (p Pace) With(o Overrides) (Pace, error) {
	for _, setting := range []struct {
		text string
		into *time.Duration
	}{{o.Idle, &p.Idle}, {o.Tail, &p.Tail}, {o.NightInterval, &p.Night.Interval}, {o.NightPeek, &p.Night.Peek}} {
		if setting.text == "" {
			continue
		}
		d, err := time.ParseDuration(setting.text)
		if err != nil {
			return p, fmt.Errorf("%q is not a duration like 90s or 2m: %w", setting.text, err)
		}
		*setting.into = d
	}
	if o.Night != "" {
		from, to, err := ParseNight(o.Night)
		if err != nil {
			return p, err
		}
		p.Night.From, p.Night.To = from, to
	}
	return p, p.Validate()
}

// DefaultPace is the pace of the badge.
var DefaultPace = Pace{
	Idle: defaultIdle,
	Tail: defaultTail,
	Night: Night{
		From:     defaultNightFrom,
		To:       defaultNightTo,
		Interval: defaultNightInterval,
		Peek:     defaultNightPeek,
	},
}

// Kind says why a picture is being shown.
type Kind int

const (
	// Burst is one of the pictures that come back to back, from feeding the
	// fish until a while after it has eaten everything.
	Burst Kind = iota
	// Settle is the last picture of a burst, and the first one at start-up.
	// The screen is going to be left alone for a while, so this is the moment
	// for it to clean itself.
	Settle
	// Ambient is a picture of the tank some time after the last one, with
	// nobody feeding the fish.
	Ambient
)

// Event is what ended a Wait.
type Event int

const (
	// Timeout means the time was up.
	Timeout Event = iota
	// Pressed means the feed button was pressed.
	Pressed
	// Quit means the program should end. Only a computer ever says so.
	Quit
)

// Env is what Run needs from the outside world.
type Env interface {
	// Now is the time.
	Now() time.Time
	// Wait sleeps for up to d, not at all if d is zero, and returns early if
	// the button is pressed or the program is asked to quit.
	Wait(d time.Duration) Event
	// Clock is the time on the wall, the one that tells night from day. Only
	// the time of day matters. ok is false if there is no way to tell, and
	// then the tank never goes to sleep for the night.
	Clock() (t time.Time, ok bool)
}

// startOfTime is when the tank's clock starts. It only has to be a time.
var startOfTime = time.Unix(1_000_000_000, 0)

// App is the aquarium on the badge's grid of characters.
type App struct {
	anim   *anim.Animation
	tank   *aquarium.Aquarium
	clock  time.Time
	asleep bool // it is night
	frames int  // how many frames of RunLive have been shown

	peekLeft time.Duration // how much is left of the peek RunLive is showing
}

// New creates the tank. The seed decides how it looks: the same seed always
// gives the same tank and the same fish.
func New(seed1, seed2 uint64) *App {
	p := &App{
		anim:  anim.New(epd.Cols, epd.Rows),
		clock: startOfTime,
	}
	p.anim.Now = func() time.Time { return p.clock }
	rng := rand.New(rand.NewPCG(seed1, seed2))
	p.tank = aquarium.New(p.anim, aquarium.Options{CellAspect: float64(epd.CellH) / epd.CellW}, rng)
	p.tank.Populate()
	return p
}

// Rebuild throws the tank away and makes a new one.
func (p *App) Rebuild() {
	p.anim.RemoveAll()
	p.tank.Populate()
}

// Feed drops food into the tank.
func (p *App) Feed() { p.tank.DropFood() }

// Asleep reports whether it is night in the tank.
func (p *App) Asleep() bool { return p.asleep }

// Sleep makes it night in the tank: the fish is gone and what is left is the
// algae and the bubbles that slowly rise.
func (p *App) Sleep() {
	p.asleep = true
	p.tank.SetNight(true)
}

// WakeUp makes it day in the tank again.
func (p *App) WakeUp() {
	p.asleep = false
	p.tank.SetNight(false)
}

// NightUpdate moves the sleeping tank on by one picture: a bubble may start to
// rise, and the algae sways.
func (p *App) NightUpdate() {
	p.tank.NightBubble()
	p.advance(aquarium.NightStepsPerUpdate)
}

// Active reports whether the fish has something to go for: there is food in
// the tank.
func (p *App) Active() bool { return p.tank.FoodCount() > 0 }

// advance lets the tank live for a number of animation steps.
func (p *App) advance(steps int) {
	for range steps {
		p.clock = p.clock.Add(StepDuration)
		p.anim.Animate()
	}
}

// Step advances the tank by the time between two pictures of a burst.
func (p *App) Step() { p.advance(StepsPerUpdate) }

// Frame advances the tank by one animation step, which is what a frame of the
// real-time view is. At night the algae sways and, one frame in each
// aquarium.NightStepsPerUpdate, a bubble may start to rise, so that the night
// has as many bubbles in the water as the badge's pictures of it.
func (p *App) Frame() {
	if p.asleep && p.frames%aquarium.NightStepsPerUpdate == 0 {
		p.tank.NightBubble()
	}
	p.frames++
	p.advance(1)
}

// PeekLeft is how much is left of the peek at the day that RunLive is showing
// at night, or zero if there is none.
func (p *App) PeekLeft() time.Duration { return p.peekLeft }

// Drift lets the tank live for d, as much as it would in the real world: the
// fish goes wherever it wants, the algae sways, bubbles come and go.
func (p *App) Drift(d time.Duration) { p.advance(int(d / StepDuration)) }

// Draw draws the tank on a canvas.
func (p *App) Draw(c anim.Canvas) { p.anim.Draw(c) }

// Render draws the tank on a frame for the e-ink screen: black on white by day,
// and the negative of that, white on black, at night.
func (p *App) Render(f *epd.Frame) {
	p.Draw(epd.Canvas{F: f})
	if p.asleep {
		f.Invert()
	}
}

// waitIgnoringTheButton waits for d, whatever is pressed meanwhile, and says
// whether the program was asked to quit.
func waitIgnoringTheButton(env Env, d time.Duration) (quit bool) {
	end := env.Now().Add(d)
	for {
		left := end.Sub(env.Now())
		if left <= 0 {
			return false
		}
		switch env.Wait(left) {
		case Quit:
			return true
		case Timeout:
			return false
		}
	}
}

// Run runs the tank until env says to quit, which on the badge is never.
//
// While nothing is going on it waits pace.Idle, lets the tank live that long
// and shows it. Pressing the button feeds the fish, and from then on pictures
// come back to back while there is food in the tank and for pace.Tail more,
// after which the tank is shown once more as the last picture of the burst
// and the slow pace starts again.
//
// At night, between pace.Night.From and pace.Night.To by the clock, the tank
// goes to sleep: it shows one last picture of the day, and from then on one
// every pace.Night.Interval, of the algae and the bubbles, in negative, until
// the night is over. Pressing the button at night is a peek at the day: the
// tank wakes up for pace.Night.Peek, with one picture of it, and nobody can
// feed the fish; then it goes back to sleep. That is two pictures of the screen
// instead of the dozens that a feeding takes. With no peek the button does
// nothing at night.
//
// present is called every time there is a picture to show; it is expected to
// take as long as the screen takes to refresh.
func (p *App) Run(env Env, pace Pace, present func(Kind)) {
	if night, _ := pace.Night.state(env); night {
		p.Sleep() // start the way it will go on
	}
	present(Settle)

	var (
		busyUntil time.Time // pictures come back to back until then...
		bursts    int       // ... and this many have, in a row
		wasBusy   bool
	)
	for {
		busy := (p.Active() || env.Now().Before(busyUntil)) && bursts < MaxBurstUpdates
		if !busy {
			if wasBusy {
				wasBusy, bursts = false, 0
				present(Settle)
			}
			night, left := pace.Night.state(env)
			switch {
			case night && !p.asleep:
				p.Sleep()
				present(Settle) // leave a clean picture for the night
			case !night && p.asleep:
				p.WakeUp()
				p.Drift(pace.Idle)
				present(Ambient)
			}
			if p.asleep {
				switch env.Wait(min(pace.Night.Interval, left)) { // and wake up on time
				case Quit:
					return
				case Pressed:
					if pace.Night.Peek > 0 {
						p.WakeUp()
						present(Ambient) // the tank by day, with the fish where it was put
						if waitIgnoringTheButton(env, pace.Night.Peek) {
							return
						}
						// it goes back to sleep at the top of the loop
					}
				case Timeout:
					if night, _ := pace.Night.state(env); night {
						p.NightUpdate()
						present(Ambient)
					}
				}
				continue
			}
			switch env.Wait(pace.Idle) {
			case Quit:
				return
			case Pressed:
				p.Feed()
			case Timeout:
				p.Drift(pace.Idle)
				present(Ambient)
			}
			continue
		}

		wasBusy = true
		p.Step()
		if p.Active() {
			busyUntil = env.Now().Add(pace.Tail)
		}
		bursts++
		present(Burst)

		// Look at the button between pictures without waiting for it.
		switch env.Wait(0) {
		case Quit:
			return
		case Pressed:
			p.Feed()
		}
	}
}

// RunLive runs the tank in real time, until env says to quit: an animation step
// every StepDuration, which is how the scene was made to be seen, with draw
// called after each one. The badge's screen takes a second to refresh and can't
// do this; it is for watching the tank on a computer, to see that it works.
//
// It is day and night by the clock as on the badge, and the button feeds the
// fish by day. At night it is a peek at the day for night.Peek, with no
// feeding, as on the badge; with no peek the button does nothing at night.
//
// A press that feeds the fish is drawn at once, between two steps.
//
// If draw is too slow the tank slows down with it instead of rushing to catch
// up.
func (p *App) RunLive(env Env, night Night, draw func()) {
	var peekUntil time.Time
	next := env.Now()
	for {
		clockNight, _ := night.state(env)
		if !clockNight {
			peekUntil = time.Time{} // a peek ends with the night
		}
		peeking := clockNight && env.Now().Before(peekUntil)
		p.peekLeft = 0
		if peeking {
			p.peekLeft = peekUntil.Sub(env.Now())
		}
		switch asleep := clockNight && !peeking; {
		case asleep && !p.asleep:
			p.Sleep()
		case !asleep && p.asleep:
			p.WakeUp()
		}
		p.Frame()
		draw()

		next = next.Add(StepDuration)
		if now := env.Now(); now.After(next) {
			next = now // behind: do not try to catch up
		}
		for {
			switch env.Wait(max(next.Sub(env.Now()), 0)) {
			case Quit:
				return
			case Pressed:
				switch {
				case !clockNight:
					p.Feed()
					draw() // now, or a fish that is right there eats them before they are seen
				case !peeking && night.Peek > 0:
					peekUntil = env.Now().Add(night.Peek)
				}
				continue // and wait for the rest of the step
			}
			break
		}
	}
}
