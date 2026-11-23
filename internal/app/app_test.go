// SPDX-License-Identifier: GPL-2.0-or-later

package app

import (
	"slices"
	"sort"
	"testing"
	"time"

	"go-asciiquarium/internal/epd"
)

// testRefresh is how long the screen takes to refresh in these tests.
const testRefresh = time.Second

// world is a simulated outside world for Run: a clock that moves only when
// the code waits or the screen refreshes, a button pressed at given times, and
// a moment when the program is asked to quit.
type world struct {
	now     time.Time
	presses []time.Time
	quitAt  time.Time

	// startsAt is the time of day on the wall when the world begins.
	startsAt time.Duration
	// noClock means nobody can tell what time of day it is.
	noClock bool
}

func newWorld(quitAfter time.Duration, pressesAfter ...time.Duration) *world {
	w := &world{now: time.Unix(0, 0)}
	w.quitAt = w.now.Add(quitAfter)
	for _, d := range pressesAfter {
		w.presses = append(w.presses, w.now.Add(d))
	}
	slices.SortFunc(w.presses, func(a, b time.Time) int { return a.Compare(b) })
	return w
}

func (w *world) Now() time.Time { return w.now }

// Clock is the time on the wall: the world began at startsAt on some day.
func (w *world) Clock() (time.Time, bool) {
	start := time.Date(2026, time.November, 23, 0, 0, 0, 0, time.UTC).Add(w.startsAt)
	return start.Add(w.now.Sub(time.Unix(0, 0))), !w.noClock
}

// Wait lets time pass until the first of: the time is up, the button is
// pressed, the program is asked to quit. What already happened counts as now.
func (w *world) Wait(d time.Duration) Event {
	at, event := w.now.Add(d), Timeout
	if len(w.presses) > 0 && !w.presses[0].After(at) {
		at, event = w.presses[0], Pressed
	}
	if !w.quitAt.After(at) {
		at, event = w.quitAt, Quit
	}
	if event == Pressed {
		w.presses = w.presses[1:]
	}
	w.now = later(w.now, at)
	return event
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// picture is a picture that Run asked to show.
type picture struct {
	kind   Kind
	at     time.Time
	active bool // there was food in the tank
}

// run runs the tank in a world until the world says to quit, and returns the
// pictures it showed. Showing one takes testRefresh.
func run(p *App, w *world, pace Pace) []picture {
	var out []picture
	p.Run(w, pace, func(k Kind) {
		out = append(out, picture{k, w.now, p.Active()})
		w.now = w.now.Add(testRefresh)
	})
	return out
}

var testPace = Pace{Idle: 30 * time.Second, Tail: 30 * time.Second}

// kinds lists what kind each picture is.
func kinds(pics []picture) []Kind {
	out := make([]Kind, len(pics))
	for i, p := range pics {
		out[i] = p.kind
	}
	return out
}

func TestBootPictureHasTheWaterSurface(t *testing.T) {
	p := New(1, 2)
	var f epd.Frame
	p.Render(&f)

	// A row of waves across the top: every cell of row 0 is a '~'.
	var wave, blank epd.Frame
	for c := 0; c < epd.Cols; c++ {
		wave.DrawGlyph(c, 0, '~')
	}
	for y := 0; y < epd.CellH; y++ {
		for x := 0; x < epd.Width; x++ {
			if f.Black(x, y) != wave.Black(x, y) {
				t.Fatalf("top row differs from a row of '~' at pixel (%d,%d)", x, y)
			}
		}
	}
	if f.Equal(&blank) {
		t.Error("the picture is empty")
	}
}

// runes is a Canvas that collects every character drawn.
type runes struct{ seen map[rune]bool }

func (r *runes) Clear() {}
func (r *runes) SetCell(x, y int, ch rune) {
	if x < 0 || y < 0 || x >= epd.Cols || y >= epd.Rows {
		panic("cell outside the grid")
	}
	r.seen[ch] = true
}

func TestEveryCharacterDrawnIsInTheFont(t *testing.T) {
	seen := &runes{seen: map[rune]bool{}}
	for seed := uint64(1); seed <= 10; seed++ {
		p := New(seed, 7)
		w := newWorld(20*time.Minute, time.Minute, 8*time.Minute)
		p.Run(w, testPace, func(Kind) { p.Draw(seen); w.now = w.now.Add(testRefresh) })
	}
	var missing []string
	for r := range seen.seen {
		if !epd.HasGlyph(r) {
			missing = append(missing, string(r))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("no glyph for %q", missing)
	}
	// The parts of the scene that are always there show up: waves, algae and
	// their blanks, the first stage of a bubble, food, and the fish (whose
	// characters all appear whichever way it faces, so does '<' and '>').
	// The full-grown bubble 'O' needs some luck, so it is not checked.
	for _, r := range "~() .o*<>" {
		if !seen.seen[r] {
			t.Errorf("%q was never drawn", r)
		}
	}
}

func TestNothingToDoAtFirst(t *testing.T) {
	if New(1, 2).Active() {
		t.Error("a new tank is active")
	}
}

func TestIdleShowsOnePictureEveryIdleInterval(t *testing.T) {
	pics := run(New(1, 2), newWorld(5*time.Minute), testPace)

	if pics[0].kind != Settle {
		t.Errorf("the first picture is a %v, want Settle", pics[0].kind)
	}
	for i, pic := range pics[1:] {
		if pic.kind != Ambient {
			t.Errorf("picture %d is a %v, want Ambient", i+1, pic.kind)
		}
	}
	// Each one comes an idle interval after the screen was done with the last.
	for i := 1; i < len(pics); i++ {
		if got, want := pics[i].at.Sub(pics[i-1].at), testPace.Idle+testRefresh; got != want {
			t.Errorf("picture %d came %v after the one before, want %v", i, got, want)
		}
	}
	if want := int((5 * time.Minute) / (testPace.Idle + testRefresh)); len(pics) < want {
		t.Errorf("%d pictures in 5 minutes, want at least %d", len(pics), want)
	}
}

func TestFeedingSpeedsUpThenSlowsDownAgain(t *testing.T) {
	const pressAt = 95 * time.Second
	pics := run(New(3, 4), newWorld(10*time.Minute, pressAt), testPace)

	// Find the burst: from the first picture after the press to the Settle.
	first := slices.IndexFunc(pics, func(p picture) bool { return p.kind == Burst })
	settle := slices.IndexFunc(pics[first:], func(p picture) bool { return p.kind == Settle }) + first
	if first < 0 || settle <= first {
		t.Fatalf("no burst in %v", kinds(pics))
	}

	// Before it: the slow pace.
	for i := 1; i < first; i++ {
		if pics[i].kind != Ambient {
			t.Errorf("picture %d before the feeding is a %v", i, pics[i].kind)
		}
	}
	if at := pics[first].at; at.Before(time.Unix(0, 0).Add(pressAt)) {
		t.Errorf("the burst starts at %v, before the press", at)
	}

	// During it: back to back, as fast as the screen refreshes.
	for i := first + 1; i < settle; i++ {
		if pics[i].kind != Burst {
			t.Errorf("picture %d in the burst is a %v", i, pics[i].kind)
		}
		if got := pics[i].at.Sub(pics[i-1].at); got != testRefresh {
			t.Errorf("burst picture %d came %v after the one before, want %v", i, got, testRefresh)
		}
	}

	// It goes on for the tail after the last picture with food in the tank,
	// and the food is gone when it ends.
	lastFood := slices.IndexFunc(pics[first:settle], func(p picture) bool { return !p.active }) + first - 1
	if lastFood < first {
		t.Fatalf("the burst never had food? %v", kinds(pics[first:settle]))
	}
	for i := first; i <= lastFood; i++ {
		if !pics[i].active {
			t.Errorf("picture %d: the food was gone, then back", i)
		}
	}
	tailPictures := settle - 1 - lastFood
	if want := int(testPace.Tail/testRefresh) - 1; tailPictures != want {
		t.Errorf("%d pictures after the last food, want %d (a tail of %v at %v each)", tailPictures, want, testPace.Tail, testRefresh)
	}
	if pics[settle].active {
		t.Error("food left when the burst ended")
	}

	// After it: the slow pace again, starting an idle interval after the screen
	// was done with the Settle.
	if next := pics[settle+1]; next.kind != Ambient || next.at.Sub(pics[settle].at) != testPace.Idle+testRefresh {
		t.Errorf("after the burst: a %v after %v", next.kind, next.at.Sub(pics[settle].at))
	}
	t.Logf("burst of %d pictures: %d with food, then %d in the tail", settle-first, lastFood-first+1, tailPictures)
}

func TestThereIsOneSettlePerBurst(t *testing.T) {
	pics := run(New(5, 6), newWorld(30*time.Minute, 2*time.Minute, 12*time.Minute, 20*time.Minute), testPace)
	settles := 0
	for _, p := range pics {
		if p.kind == Settle {
			settles++
		}
	}
	if want := 1 + 3; settles != want { // the first picture, and the end of each of the 3 bursts
		t.Errorf("%d Settle pictures, want %d: %v", settles, want, kinds(pics))
	}
}

func TestFeedingAgainWhileTheyComeBackToBackKeepsThemComing(t *testing.T) {
	const firstPress = 95 * time.Second
	alone := run(New(3, 4), newWorld(10*time.Minute, firstPress), testPace)
	endOfBurst := func(pics []picture) time.Time {
		i := slices.IndexFunc(pics[1:], func(p picture) bool { return p.kind == Settle })
		return pics[i+1].at
	}
	end := endOfBurst(alone)

	// A second press in the tail, when the fish has long eaten everything.
	again := run(New(3, 4), newWorld(10*time.Minute, firstPress, end.Sub(time.Unix(0, 0))-10*time.Second), testPace)
	if got := endOfBurst(again); got.Sub(end) < testPace.Tail/2 {
		t.Errorf("the burst ended at %v, the one without the second press at %v: not extended", got, end)
	}
}

func TestQuitStopsRightAway(t *testing.T) {
	pics := run(New(1, 2), newWorld(10*time.Second), testPace)
	if len(pics) != 1 {
		t.Errorf("%d pictures, want only the first", len(pics))
	}
	pics = run(New(1, 2), newWorld(100*time.Second, 5*time.Second), testPace) // quits during a burst
	if last := pics[len(pics)-1]; last.at.After(time.Unix(0, 0).Add(100 * time.Second)) {
		t.Errorf("a picture at %v, after the quit", last.at)
	}
}

func TestABurstIsBounded(t *testing.T) {
	// A tail that never ends must not mean refreshing the screen forever.
	endless := Pace{Idle: testPace.Idle, Tail: 100 * 24 * time.Hour}
	pics := run(New(1, 2), newWorld(20*time.Minute, 5*time.Second), endless)
	bursts := 0
	for _, p := range pics[1:] {
		if p.kind != Burst {
			break
		}
		bursts++
	}
	if bursts != MaxBurstUpdates {
		t.Errorf("%d pictures back to back, want %d", bursts, MaxBurstUpdates)
	}
	if next := pics[1+bursts]; next.kind != Settle {
		t.Errorf("a %v after the burst, want Settle", next.kind)
	}
}

func TestADriftLetsTheTankLiveAsLongAsItWasLeftAlone(t *testing.T) {
	p := New(4, 4)
	start := p.clock
	p.Drift(testPace.Idle)
	if got := p.clock.Sub(start); got != testPace.Idle {
		t.Errorf("the tank lived %v, want %v", got, testPace.Idle)
	}
}

func TestAStepMovesTheClockByTheTimeBetweenBurstPictures(t *testing.T) {
	p := New(4, 4)
	start := p.clock
	p.Step()
	if got, want := p.clock.Sub(start), StepsPerUpdate*StepDuration; got != want {
		t.Errorf("one step moved the clock by %v, want %v", got, want)
	}
}

func TestAFishLeftAloneEndsUpSomewhereElse(t *testing.T) {
	// The tank is not the same picture every time: the fish swims about.
	p := New(7, 8)
	var frames []epd.Frame
	w := newWorld(30 * time.Minute)
	p.Run(w, testPace, func(Kind) {
		var f epd.Frame
		p.Render(&f)
		frames = append(frames, f)
		w.now = w.now.Add(testRefresh)
	})
	different := 0
	for i := 1; i < len(frames); i++ {
		if !frames[i].Equal(&frames[i-1]) {
			different++
		}
	}
	if different < len(frames)*3/4 {
		t.Errorf("only %d of %d pictures differ from the one before", different, len(frames)-1)
	}
}

func TestLongIdleKeepsTheTankAlive(t *testing.T) {
	// A day of sitting on a desk: the algae dies and grows back, bubbles come
	// and go, and nothing runs away or piles up.
	p := New(9, 10)
	before := p.anim.EntityCount()
	pics := run(p, newWorld(24*time.Hour), testPace)
	t.Logf("%d pictures in a day", len(pics))
	if got := p.anim.EntityCount(); got < before || got > before+bubblesAtMost {
		t.Errorf("%d things in the tank after a day, %d at the start", got, before)
	}
}

// bubblesAtMost is how many bubbles can be in the tank at once, with room to spare.
const bubblesAtMost = epd.Rows

func TestOnlyTheScreenChangesWhenSomethingHappens(t *testing.T) {
	sim := &epd.Sim{}
	display := epd.New(sim, epd.Config{})
	p := New(9, 9)
	var f epd.Frame
	w := newWorld(10*time.Minute, 3*time.Minute)
	p.Run(w, testPace, func(k Kind) {
		p.Render(&f)
		display.Show(&f, k == Settle)
		w.now = w.now.Add(testRefresh)
	})

	// Every refresh was for a picture or a cleaning, and nothing else.
	s := display.Stats()
	const refreshesPerCleaning = 2 // one black, one white
	if sim.Refreshes != s.Pictures+refreshesPerCleaning*s.Cleans {
		t.Errorf("%d refreshes for %d pictures and %d cleanings", sim.Refreshes, s.Pictures, s.Cleans)
	}
	t.Logf("%d pictures, %d skipped, %d cleanings, %d refreshes in 10 minutes", s.Pictures, s.Skipped, s.Cleans, sim.Refreshes)
}

func TestSameSeedSameTank(t *testing.T) {
	last := func() epd.Frame {
		p := New(11, 22)
		run(p, newWorld(10*time.Minute, time.Minute), testPace)
		var f epd.Frame
		p.Render(&f)
		return f
	}
	a, b := last(), last()
	if !a.Equal(&b) {
		t.Error("the same seed gave two different tanks")
	}
}

func TestDifferentSeedsDifferentTanks(t *testing.T) {
	var a, b epd.Frame
	New(1, 1).Render(&a)
	New(2, 2).Render(&b)
	if a.Equal(&b) {
		t.Error("different seeds gave the same tank")
	}
}

func TestFeedAgain(t *testing.T) {
	p := New(5, 5)
	p.Feed()
	p.Step()
	before := p.tank.FoodCount()
	p.Feed()
	if p.tank.FoodCount() <= before {
		t.Error("feeding again added no food")
	}
	if !p.Active() {
		t.Error("not active with food in the tank")
	}
}

func TestRebuild(t *testing.T) {
	p := New(3, 3)
	p.Feed()
	p.Rebuild()
	if p.Active() {
		t.Error("a rebuilt tank still has food")
	}
	var f, blank epd.Frame
	p.Render(&f)
	if f.Equal(&blank) {
		t.Error("the rebuilt tank is empty")
	}
}

// Times of day.
func hours(h int) time.Duration { return time.Duration(h) * time.Hour }

func clockAt(h, m, sec int) time.Time {
	return time.Date(2026, time.November, 23, h, m, sec, 0, time.UTC)
}

// nightPace has the night from ten in the evening to seven in the morning.
var nightPace = Pace{
	Idle:  30 * time.Second,
	Tail:  30 * time.Second,
	Night: Night{From: hours(22), To: hours(7), Interval: 10 * time.Minute},
}

// worldAt is a world that starts at a time of day.
func worldAt(startsAt time.Duration, quitAfter time.Duration, pressesAfter ...time.Duration) *world {
	w := newWorld(quitAfter, pressesAfter...)
	w.startsAt = startsAt
	return w
}

func TestNightFallsBetweenTheTwoTimes(t *testing.T) {
	through := Night{From: hours(22), To: hours(7), Interval: time.Minute} // through midnight
	within := Night{From: hours(1), To: hours(5), Interval: time.Minute}   // inside one day
	for _, tt := range []struct {
		name  string
		night Night
		at    time.Time
		want  bool
	}{
		{"the evening before", through, clockAt(21, 59, 59), false},
		{"night begins", through, clockAt(22, 0, 0), true},
		{"before midnight", through, clockAt(23, 59, 59), true},
		{"midnight", through, clockAt(0, 0, 0), true},
		{"before morning", through, clockAt(6, 59, 59), true},
		{"night ends", through, clockAt(7, 0, 0), false},
		{"midday", through, clockAt(12, 0, 0), false},
		{"before it begins", within, clockAt(0, 59, 59), false},
		{"inside begins", within, clockAt(1, 0, 0), true},
		{"inside, late", within, clockAt(4, 59, 59), true},
		{"inside ends", within, clockAt(5, 0, 0), false},
		{"inside, midday", within, clockAt(12, 0, 0), false},
		{"inside, evening", within, clockAt(23, 0, 0), false},
		{"no night: the same time twice", Night{From: hours(3), To: hours(3), Interval: time.Minute}, clockAt(3, 0, 0), false},
		{"no night: no interval", Night{From: hours(22), To: hours(7)}, clockAt(23, 0, 0), false},
	} {
		if got := tt.night.At(tt.at); got != tt.want {
			t.Errorf("%s: At(%v) = %v, want %v", tt.name, tt.at.Format("15:04:05"), got, tt.want)
		}
	}
}

func TestHowMuchIsLeftOfTheNight(t *testing.T) {
	through := Night{From: hours(22), To: hours(7), Interval: time.Minute}
	within := Night{From: hours(1), To: hours(5), Interval: time.Minute}
	for _, tt := range []struct {
		night Night
		at    time.Time
		want  time.Duration
	}{
		{through, clockAt(22, 0, 0), 9 * time.Hour},
		{through, clockAt(23, 30, 0), 7*time.Hour + 30*time.Minute},
		{through, clockAt(0, 0, 0), 7 * time.Hour},
		{through, clockAt(6, 59, 30), 30 * time.Second},
		{within, clockAt(1, 0, 0), 4 * time.Hour},
		{within, clockAt(4, 0, 0), time.Hour},
	} {
		if got := tt.night.untilEnd(tt.at); got != tt.want {
			t.Errorf("untilEnd(%v) = %v, want %v", tt.at.Format("15:04:05"), got, tt.want)
		}
	}
}

func blackShare(f *epd.Frame) float64 {
	black := 0
	for y := 0; y < epd.Height; y++ {
		for x := 0; x < epd.Width; x++ {
			if f.Black(x, y) {
				black++
			}
		}
	}
	return float64(black) / float64(epd.Width*epd.Height)
}

func TestTheNightBeginsAndEndsAtTheGivenTimes(t *testing.T) {
	// Starts at nine in the evening and goes on through the night into the morning.
	w := worldAt(hours(21), 13*time.Hour)
	pics := run(New(1, 2), w, nightPace)
	at := func(h int) time.Time { return time.Unix(0, 0).Add(hours(h - 21)) } // when the clock says h o'clock

	// By day before it: the usual pace.
	enter := slices.IndexFunc(pics[1:], func(p picture) bool { return p.kind == Settle }) + 1
	if enter < 1 {
		t.Fatalf("it never went to sleep: %v", kinds(pics[:10]))
	}
	for i, p := range pics[1:enter] {
		if p.kind != Ambient {
			t.Errorf("picture %d before the night is a %v", i+1, p.kind)
		}
	}
	// It goes to sleep at the first look after ten, which is within one idle period.
	if got := pics[enter].at; got.Before(at(22)) || got.After(at(22).Add(nightPace.Idle+testRefresh)) {
		t.Errorf("it went to sleep at %v, night begins at %v", got.Sub(at(21)), hours(1))
	}

	// All night: a picture every Interval, and no other kind.
	wake := slices.IndexFunc(pics[enter+1:], func(p picture) bool { return p.at.After(at(7+24)) || !p.at.Before(at(7+24)) }) + enter + 1
	if wake <= enter+1 {
		t.Fatalf("it never woke up: %v", kinds(pics[enter:]))
	}
	for i := enter + 1; i < wake; i++ {
		if pics[i].kind != Ambient {
			t.Errorf("picture %d in the night is a %v", i, pics[i].kind)
		}
		if got, want := pics[i].at.Sub(pics[i-1].at), nightPace.Night.Interval+testRefresh; got != want {
			t.Errorf("night picture %d came %v after the one before, want %v", i, got, want)
		}
	}
	// It wakes up when the night ends, to the second, not at the next picture.
	if got := pics[wake].at; !got.Equal(at(7 + 24)) {
		t.Errorf("it woke up at %v, the night ends at %v", got.Sub(at(21)), at(7+24).Sub(at(21)))
	}
	if night := wake - enter - 1; night < 50 || night > 56 { // nine hours, ten minutes apart
		t.Errorf("%d pictures in the night, want about 54", night)
	}
	// And the usual pace again.
	if next := pics[wake+1]; next.kind != Ambient || next.at.Sub(pics[wake].at) != nightPace.Idle+testRefresh {
		t.Errorf("after the night: a %v after %v", next.kind, next.at.Sub(pics[wake].at))
	}
	t.Logf("%d pictures from 21:00 to 10:00; the night's pictures: %d", len(pics), wake-enter-1)
}

func TestNightIsInNegativeWithoutTheFishOrTheSurface(t *testing.T) {
	p := New(3, 4)
	w := worldAt(hours(20), 6*time.Hour)
	var day, night int
	var frame epd.Frame
	p.Run(w, nightPace, func(Kind) {
		p.Render(&frame)
		fish := len(p.anim.EntitiesOfType("fish"))
		surface := len(p.anim.EntitiesOfType("waterline"))
		share := blackShare(&frame)
		if p.Asleep() {
			night++
			if fish != 0 || surface != 0 {
				t.Errorf("at night: %d fish, %d water surfaces", fish, surface)
			}
			// The negative of a picture that is nearly all white is nearly all
			// black, but the algae and the bubbles are white on it.
			if share < .9 || share == 1 {
				t.Errorf("a night picture is %.0f%% black", share*100)
			}
		} else {
			day++
			if fish != 1 || surface != 1 {
				t.Errorf("by day: %d fish, %d water surfaces", fish, surface)
			}
			if share > .1 {
				t.Errorf("a day picture is %.0f%% black", share*100)
			}
		}
		w.now = w.now.Add(testRefresh)
	})
	if day == 0 || night == 0 {
		t.Errorf("%d day pictures and %d night pictures: both were expected", day, night)
	}
}

func TestStartingAtNightStartsWithANightPicture(t *testing.T) {
	p := New(1, 2)
	w := worldAt(hours(23), time.Hour)
	var first *bool
	var frame epd.Frame
	var share float64
	p.Run(w, nightPace, func(Kind) {
		if first == nil {
			asleep := p.Asleep()
			first = &asleep
			p.Render(&frame)
			share = blackShare(&frame)
		}
		w.now = w.now.Add(testRefresh)
	})
	if first == nil || !*first || share < .9 {
		t.Errorf("the first picture at 23:00 is not a night picture (asleep: %v, %.0f%% black)", first, share*100)
	}
}

// peekPace is the night of nightPace with a peek of half a minute.
var peekPace = func() Pace {
	p := nightPace
	p.Night.Peek = 30 * time.Second
	return p
}()

// asleepAt records, for each picture, whether the tank was asleep in it and
// how much food it held.
type nightPicture struct {
	kind   Kind
	at     time.Time
	asleep bool
	fish   int
	food   int
}

func runRecordingNight(p *App, w *world, pace Pace) []nightPicture {
	var out []nightPicture
	p.Run(w, pace, func(k Kind) {
		out = append(out, nightPicture{k, w.now, p.Asleep(), len(p.anim.EntitiesOfType("fish")), p.tank.FoodCount()})
		w.now = w.now.Add(testRefresh)
	})
	return out
}

func TestPressingTheButtonAtNightIsAPeekAtTheDay(t *testing.T) {
	const pressAt = 90 * time.Minute
	p := New(1, 2)
	w := worldAt(hours(23), 3*time.Hour, pressAt)
	seen := runRecordingNight(p, w, peekPace)

	// Which pictures came from the press: those between it and the next picture
	// of the night at its own pace.
	pressTime := time.Unix(0, 0).Add(pressAt)
	var peek []nightPicture
	for _, pic := range seen {
		if !pic.at.Before(pressTime) && pic.at.Before(pressTime.Add(peekPace.Night.Peek+10*time.Second)) {
			peek = append(peek, pic)
		}
	}
	if len(peek) != 2 {
		t.Fatalf("the press made %d pictures: %v, want the day and the night again", len(peek), peek)
	}
	day, night := peek[0], peek[1]
	if day.kind != Ambient || day.asleep || day.fish != 1 {
		t.Errorf("the peek is %+v: want one picture of the tank by day, with its fish", day)
	}
	if night.kind != Settle || !night.asleep || night.fish != 0 {
		t.Errorf("after the peek: %+v, want it asleep again, with a clean Settle", night)
	}
	if got := night.at.Sub(day.at); got < testRefresh+peekPace.Night.Peek {
		t.Errorf("the day was shown for %v, want at least the peek of %v", got-testRefresh, peekPace.Night.Peek)
	}
	for _, pic := range seen {
		if pic.kind == Burst || pic.food > 0 {
			t.Fatalf("the fish was fed at night: %+v", pic)
		}
	}
	// The night goes on at its own pace afterwards.
	last := seen[len(seen)-1]
	if last.kind != Ambient || !last.asleep {
		t.Errorf("the last picture is %+v, want a night one", last)
	}
}

func TestAPeekIsIgnoredWhenItsOverTheButtonIsPressedDuringIt(t *testing.T) {
	const pressAt = 90 * time.Minute
	p := New(1, 2)
	w := worldAt(hours(23), 3*time.Hour, pressAt, pressAt+10*time.Second, pressAt+20*time.Second)
	seen := runRecordingNight(p, w, peekPace)
	var days int
	for _, pic := range seen {
		if !pic.asleep {
			days++
		}
		if pic.kind == Burst || pic.food > 0 {
			t.Fatalf("the fish was fed at night: %+v", pic)
		}
	}
	if days != 1 {
		t.Errorf("%d pictures of the day, want exactly one: the presses during the peek do nothing", days)
	}
	// And the peek did not get longer: it goes back to sleep after the first press's 30 s.
	var sleptAgain time.Time
	for _, pic := range seen {
		if pic.kind == Settle && pic.asleep && pic.at.After(time.Unix(0, 0).Add(pressAt)) {
			sleptAgain = pic.at
			break
		}
	}
	if limit := time.Unix(0, 0).Add(pressAt + peekPace.Night.Peek + 2*testRefresh); sleptAgain.IsZero() || sleptAgain.After(limit) {
		t.Errorf("it slept again at %v, want by %v", sleptAgain, limit)
	}
}

func TestWithoutAPeekTheButtonDoesNothingAtNight(t *testing.T) {
	quiet := nightPace // no Peek
	with := runRecordingNight(New(1, 2), worldAt(hours(23), 3*time.Hour, 90*time.Minute), quiet)
	without := runRecordingNight(New(1, 2), worldAt(hours(23), 3*time.Hour), quiet)
	// An ignored press only restarts the wait for the next picture of the night,
	// which can move one of them past the end: it must not add or take more.
	if d := len(with) - len(without); d < -1 || d > 1 {
		t.Fatalf("pressing the button changed the pictures: %d against %d", len(with), len(without))
	}
	for _, pic := range with {
		if pic.kind == Burst || !pic.asleep && pic.at.After(time.Unix(0, 0).Add(time.Minute)) {
			t.Fatalf("the tank woke up: %+v", pic)
		}
	}
}

func TestAPeekEndsWhenTheProgramIsAskedToQuit(t *testing.T) {
	const pressAt = 90 * time.Minute
	p := New(1, 2)
	w := worldAt(hours(23), pressAt+10*time.Second, pressAt)
	seen := runRecordingNight(p, w, peekPace) // returns, and does not hang
	if last := seen[len(seen)-1]; last.asleep {
		t.Errorf("quit during the peek: the last picture is %+v", last)
	}
}

func TestWithoutAClockItNeverSleeps(t *testing.T) {
	w := worldAt(hours(23), 3*time.Hour)
	w.noClock = true
	pics := run(New(1, 2), w, nightPace)
	for i, p := range pics[1:] {
		if p.kind != Ambient {
			t.Errorf("picture %d is a %v", i+1, p.kind)
		}
	}
}

func TestWithoutANightThereIsNoSleep(t *testing.T) {
	for _, night := range []Night{{}, {From: hours(3), To: hours(3), Interval: time.Minute}} {
		pace := testPace
		pace.Night = night
		pics := run(New(1, 2), worldAt(hours(3), 3*time.Hour), pace)
		for i, p := range pics[1:] {
			if p.kind != Ambient {
				t.Errorf("%+v: picture %d is a %v", night, i+1, p.kind)
			}
		}
	}
}

func TestParseNight(t *testing.T) {
	for in, want := range map[string][2]time.Duration{
		"22:00-07:00":     {hours(22), hours(7)},
		"23:30-06:15":     {23*time.Hour + 30*time.Minute, 6*time.Hour + 15*time.Minute},
		"00:00-12:00":     {0, hours(12)},
		" 22:00 - 07:00 ": {hours(22), hours(7)},
	} {
		from, to, err := ParseNight(in)
		if err != nil || from != want[0] || to != want[1] {
			t.Errorf("ParseNight(%q) = %v, %v, %v; want %v", in, from, to, err, want)
		}
	}
	for _, in := range []string{"", "22:00", "22:00-", "-07:00", "25:00-07:00", "22:60-07:00", "ten-seven", "22:00 to 07:00"} {
		if _, _, err := ParseNight(in); err == nil {
			t.Errorf("ParseNight(%q) was accepted", in)
		}
	}
}

func TestANightWritesAsItIsRead(t *testing.T) {
	n := Night{From: 23*time.Hour + 30*time.Minute, To: 6*time.Hour + 15*time.Minute, Interval: time.Minute}
	if got := n.String(); got != "23:30-06:15" {
		t.Errorf("String() = %q", got)
	}
	from, to, err := ParseNight(n.String())
	if err != nil || from != n.From || to != n.To {
		t.Errorf("reading it back gave %v %v %v", from, to, err)
	}
}

func TestPaceValidate(t *testing.T) {
	if err := DefaultPace.Validate(); err != nil {
		t.Errorf("the default pace is not valid: %v", err)
	}
	if DefaultPace.Idle != time.Minute {
		t.Errorf("the default time between pictures is %v, want a minute", DefaultPace.Idle)
	}
	if got := DefaultPace.Night.String(); got != "22:00-07:00" {
		t.Errorf("the default night is %s", got)
	}
	if DefaultPace.Night.Peek != 30*time.Second {
		t.Errorf("the default peek at night is %v, want half a minute", DefaultPace.Night.Peek)
	}
	for name, pace := range map[string]Pace{
		"no time between pictures": {Idle: 0, Tail: time.Second},
		"negative tail":            {Idle: time.Second, Tail: -time.Second},
		"negative night interval":  {Idle: time.Second, Night: Night{From: hours(1), To: hours(2), Interval: -time.Second}},
		"negative night peek":      {Idle: time.Second, Night: Night{From: hours(1), To: hours(2), Interval: time.Minute, Peek: -time.Second}},
		"a night past a day":       {Idle: time.Second, Night: Night{From: hours(1), To: hours(25), Interval: time.Minute}},
		"a night before midnight":  {Idle: time.Second, Night: Night{From: -time.Hour, To: hours(2), Interval: time.Minute}},
		"a night with no length":   {Idle: time.Second, Night: Night{From: hours(3), To: hours(3), Interval: time.Minute}},
	} {
		if pace.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// No night: its times don't matter.
	if err := (Pace{Idle: time.Second}).Validate(); err != nil {
		t.Errorf("a pace without a night: %v", err)
	}
}

func TestOverrides(t *testing.T) {
	pace, err := DefaultPace.With(Overrides{Idle: "2m", Tail: "45s", Night: "23:00-06:30", NightInterval: "5m", NightPeek: "1m"})
	if err != nil {
		t.Fatal(err)
	}
	want := Pace{Idle: 2 * time.Minute, Tail: 45 * time.Second, Night: Night{From: hours(23), To: 6*time.Hour + 30*time.Minute, Interval: 5 * time.Minute, Peek: time.Minute}}
	if pace != want {
		t.Errorf("got %+v, want %+v", pace, want)
	}
	if same, err := DefaultPace.With(Overrides{}); err != nil || same != DefaultPace {
		t.Errorf("no overrides changed the pace: %+v, %v", same, err)
	}
	for _, bad := range []Overrides{{Idle: "soon"}, {Tail: "-5s"}, {Night: "late"}, {NightInterval: "x"}, {Idle: "0s"}, {Night: "07:00-07:00"}, {NightPeek: "-1s"}, {NightPeek: "later"}} {
		if _, err := DefaultPace.With(bad); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}

// frame is what the real-time view showed in one frame.
type frame struct {
	at       time.Time
	steps    int // animation steps the tank has lived through so far
	asleep   bool
	food     int
	fish     int
	bubbles  int
	peekLeft time.Duration
}

// runLive runs the tank in real time in a world, with drawTime to draw a frame.
func runLive(p *App, w *world, night Night, drawTime time.Duration) []frame {
	var out []frame
	p.RunLive(w, night, func() {
		out = append(out, frame{
			at:       w.now,
			steps:    int(p.clock.Sub(startOfTime) / StepDuration),
			asleep:   p.Asleep(),
			food:     p.tank.FoodCount(),
			fish:     len(p.anim.EntitiesOfType("fish")),
			bubbles:  len(p.anim.EntitiesOfType("bubble")),
			peekLeft: p.PeekLeft(),
		})
		w.now = w.now.Add(drawTime)
	})
	return out
}

func TestLiveShowsAFrameForEachStepOfTheAnimation(t *testing.T) {
	const duration = 30 * time.Second
	frames := runLive(New(1, 2), newWorld(duration), Night{}, 0)
	if want := int(duration / StepDuration); len(frames) != want {
		t.Errorf("%d frames in %v, want %d", len(frames), duration, want)
	}
	for i, f := range frames {
		if f.at.Sub(time.Unix(0, 0)) != time.Duration(i)*StepDuration {
			t.Fatalf("frame %d is shown at %v, want %v", i, f.at.Sub(time.Unix(0, 0)), time.Duration(i)*StepDuration)
		}
		if f.steps != i+1 {
			t.Fatalf("frame %d is after %d steps, want one more each time: %d", i, f.steps, i+1)
		}
		if f.asleep || f.fish != 1 {
			t.Fatalf("frame %d: %+v, want the tank by day", i, f)
		}
	}
}

func TestLiveFeedsTheFishWithoutLosingTheBeat(t *testing.T) {
	const pressAt = 5*time.Second + StepDuration/2 // between two frames
	frames := runLive(New(1, 2), newWorld(40*time.Second, pressAt), Night{}, 0)
	fed, shownAtOnce := false, false
	var before *frame // the last frame of the beat
	for i, f := range frames {
		at := f.at.Sub(time.Unix(0, 0))
		if at == pressAt { // the press is drawn at once, between two frames
			shownAtOnce = f.food > 0
			continue
		}
		if before != nil && f.at.Sub(before.at) != StepDuration {
			t.Fatalf("frame %d came %v after the one before: the press changed the beat", i, f.at.Sub(before.at))
		}
		before = &frames[i]
		if at < pressAt && f.food > 0 {
			t.Fatalf("there is food at %v, before the press", at)
		}
		fed = fed || f.food > 0
	}
	if !fed {
		t.Error("the press did not drop any food")
	}
	if !shownAtOnce {
		t.Error("the pellets were not drawn at the moment of the press")
	}
	if last := frames[len(frames)-1]; last.food != 0 {
		t.Errorf("%d pellets are left after 35 s: the fish should have eaten them", last.food)
	}
}

func TestLiveGoesToSleepAndWakesUpByTheClock(t *testing.T) {
	// From 21:59:50 to 22:00:10 and from 06:59:50 to 07:00:10.
	for _, tt := range []struct {
		name       string
		startsAt   time.Duration
		asleepFrom bool
	}{
		{"evening", hours(22) - 10*time.Second, true},
		{"morning", hours(7) - 10*time.Second, false},
	} {
		frames := runLive(New(1, 2), worldAt(tt.startsAt, 20*time.Second), nightPace.Night, 0)
		for _, f := range frames {
			boundary := f.at.Sub(time.Unix(0, 0)) >= 10*time.Second
			if want := tt.asleepFrom == boundary; f.asleep != want {
				t.Fatalf("%s: asleep = %v at %v, want %v", tt.name, f.asleep, f.at.Sub(time.Unix(0, 0)), want)
			}
		}
		last := frames[len(frames)-1]
		if tt.asleepFrom != last.asleep || (last.fish == 1) == last.asleep {
			t.Errorf("%s: the last frame is %+v", tt.name, last)
		}
	}
}

// What a frame of the real-time view has in the water, on average, by day and at night.
const liveBubblesMin, liveBubblesMax = 2, 9

func TestLiveHasBubblesAtNightAndByDay(t *testing.T) {
	for _, tt := range []struct {
		name     string
		startsAt time.Duration
	}{{"day", hours(12)}, {"night", hours(23)}} {
		frames := runLive(New(1, 2), worldAt(tt.startsAt, 5*time.Minute), nightPace.Night, 0)
		var empty, total int
		steady := frames[len(frames)/10:] // after the first bubbles have risen
		for _, f := range steady {
			total += f.bubbles
			if f.bubbles == 0 {
				empty++
			}
		}
		if share := float64(empty) / float64(len(steady)); share > .03 {
			t.Errorf("%s: %.1f%% of the frames have no bubble", tt.name, share*100)
		}
		// About as many as a picture of the badge's night has (five or so), however
		// many frames there are: a bubble is let go every few frames, not each one.
		if mean := float64(total) / float64(len(steady)); mean < liveBubblesMin || mean > liveBubblesMax {
			t.Errorf("%s: %.1f bubbles in the water on average, want %v to %v", tt.name, mean, liveBubblesMin, liveBubblesMax)
		}
	}
}

func TestLiveAPeekAtNightIsTheDayForAWhileWithNoFeeding(t *testing.T) {
	const pressAt = 10 * time.Second
	night := nightPace.Night
	night.Peek = 30 * time.Second
	// A second press, in the middle of the peek, does nothing.
	frames := runLive(New(1, 2), worldAt(hours(23), 2*time.Minute, pressAt, pressAt+15*time.Second), night, 0)
	for _, f := range frames {
		since := f.at.Sub(time.Unix(0, 0)) - pressAt
		awake := since >= 0 && since < night.Peek
		if f.asleep == awake {
			t.Fatalf("asleep = %v at %v after the press, want %v", f.asleep, since, !awake)
		}
		// The time left is what the status line counts down.
		if want := night.Peek - since; awake && f.peekLeft != want || !awake && f.peekLeft != 0 {
			t.Fatalf("%v left of the peek at %v after the press, want %v", f.peekLeft, since, max(want, 0))
		}
		if f.food > 0 {
			t.Fatalf("the fish was fed at night, at %v", since)
		}
	}
}

func TestLiveWithoutAPeekTheButtonDoesNothingAtNight(t *testing.T) {
	frames := runLive(New(1, 2), worldAt(hours(23), time.Minute, 10*time.Second), nightPace.Night, 0)
	for _, f := range frames {
		if !f.asleep || f.food > 0 {
			t.Fatalf("the press changed the night: %+v", f)
		}
	}
}

func TestLiveDoesNotRushToCatchUpWhenDrawingIsSlow(t *testing.T) {
	const duration = time.Minute
	slow := 3 * StepDuration
	frames := runLive(New(1, 2), newWorld(duration), Night{}, slow)
	if want := int(duration / slow); len(frames) != want {
		t.Errorf("%d frames in %v of %v each, want %d", len(frames), duration, slow, want)
	}
	for i, f := range frames {
		if f.steps != i+1 {
			t.Fatalf("frame %d is after %d steps: a step is a step however long the drawing takes", i, f.steps)
		}
	}
}

func TestLiveQuitsRightAway(t *testing.T) {
	frames := runLive(New(1, 2), newWorld(0), Night{}, 0)
	if len(frames) != 1 {
		t.Errorf("%d frames before quitting at once, want the first", len(frames))
	}
}

func TestLiveAfterASlowStretchItDoesNotBurstToCatchUp(t *testing.T) {
	const slowFrames = 20
	slow := 5 * StepDuration
	w := newWorld(time.Minute)
	p := New(1, 2)
	var at []time.Time
	p.RunLive(w, Night{}, func() {
		at = append(at, w.now)
		if len(at) <= slowFrames {
			w.now = w.now.Add(slow) // the drawing is slow at first...
		}
	})
	for i := slowFrames + 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < StepDuration { // ... and then it is not
			t.Fatalf("frame %d came %v after the one before, less than a step: it rushed to catch up", i, gap)
		}
	}
	if len(at) < slowFrames+10 {
		t.Fatalf("only %d frames", len(at))
	}
}
