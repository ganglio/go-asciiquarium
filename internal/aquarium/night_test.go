// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"testing"

	"go-asciiquarium/internal/anim"
)

const nightWidth, nightHeight = 44, 22

func TestNightLeavesOnlyTheAlgae(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 1)
	q.DropFood()
	for range 300 { // a few bubbles
		a.Animate()
	}
	q.SetNight(true)

	if !q.Night() {
		t.Error("it is not night")
	}
	for _, typ := range []string{"fish", "waterline", "food", "bubble"} {
		if n := len(a.EntitiesOfType(typ)); n != 0 {
			t.Errorf("%d %s left at night", n, typ)
		}
	}
	if want := nightWidth / algaeSpacing; a.EntityCount() != want {
		t.Errorf("%d things in the tank at night, want the %d stalks of algae", a.EntityCount(), want)
	}

	// All that is drawn is algae.
	r := &recorder{t: t, w: nightWidth, h: nightHeight}
	a.Draw(r)
	for pos, ch := range r.cells {
		if ch != '(' && ch != ')' && ch != ' ' {
			t.Errorf("%q drawn at %v at night", ch, pos)
		}
	}
	if len(r.cells) == 0 {
		t.Error("nothing at all is drawn at night")
	}
}

func TestDayComesBack(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 1)
	algae := nightWidth / algaeSpacing
	q.SetNight(true)
	for range 200 {
		q.NightBubble()
	}
	q.SetNight(false)

	if q.Night() {
		t.Error("it is still night")
	}
	if n := len(a.EntitiesOfType("fish")); n != 1 {
		t.Errorf("%d fish in the morning", n)
	}
	if n := len(a.EntitiesOfType("waterline")); n != 1 {
		t.Errorf("%d water surfaces in the morning", n)
	}
	if n := len(a.EntitiesOfType("bubble")); n != 0 {
		t.Errorf("%d bubbles of the night are still there", n)
	}
	if want := 2 + algae; a.EntityCount() != want {
		t.Errorf("%d things in the tank, want %d", a.EntityCount(), want)
	}
}

func TestNightAndDayAreNotAddedTwice(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 1)
	before := a.EntityCount()
	q.SetNight(false) // already day
	if a.EntityCount() != before {
		t.Error("turning day into day changed the tank")
	}
	q.SetNight(true)
	q.SetNight(true)
	q.SetNight(false)
	q.SetNight(false)
	if a.EntityCount() != before {
		t.Errorf("%d things after a night, %d before", a.EntityCount(), before)
	}
}

func TestStartingOverAtNightGivesANightTank(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 1)
	q.SetNight(true)
	a.RemoveAll()
	q.Populate()
	if want := nightWidth / algaeSpacing; a.EntityCount() != want {
		t.Errorf("%d things, want just the %d stalks of algae", a.EntityCount(), want)
	}
}

func TestNightBubblesStartNowAndThen(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 2)
	q.SetNight(true)
	const tries = 4000
	started := 0
	for range tries {
		before := len(a.EntitiesOfType("bubble"))
		q.NightBubble()
		if len(a.EntitiesOfType("bubble")) > before {
			started++
		}
		a.RemoveAll() // one at a time
		q.Populate()
	}
	got := float64(started) / tries * percent
	if got < nightBubblePercent-4 || got > nightBubblePercent+4 {
		t.Errorf("a bubble started in %.1f%% of the pictures, want about %d%%", got, nightBubblePercent)
	}
}

func TestANightBubbleRisesARowAPictureAndGrowsOnTheWay(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 3)
	q.SetNight(true)
	for len(a.EntitiesOfType("bubble")) == 0 {
		q.NightBubble()
	}
	b := a.EntitiesOfType("bubble")[0]
	if !aboveAStalk(a, b) {
		t.Fatalf("the bubble starts at (%v, %v), which is not just above a stalk", b.X, b.Y)
	}
	top := int(b.Y)

	lastFrame := b.Frame()
	for picture := 1; picture <= top; picture++ {
		for range NightStepsPerUpdate {
			a.Animate()
		}
		if want := float64(top - picture); b.Y < want-1e-9 || b.Y > want+1e-9 {
			t.Fatalf("after %d pictures the bubble is at y=%v, want %v", picture, b.Y, want)
		}
		if f := b.Frame(); f < lastFrame {
			t.Fatalf("the bubble shrank from frame %d to %d at y=%v", lastFrame, f, b.Y)
		} else {
			lastFrame = f
		}
	}
	// At the top it is full size, and it leaves.
	if lastFrame != len(bubbleFrames)-1 {
		t.Errorf("the bubble is at frame %d at the top, want the last, %d", lastFrame, len(bubbleFrames)-1)
	}
	for range 2 * NightStepsPerUpdate {
		a.Animate()
	}
	if n := len(a.EntitiesOfType("bubble")); n != 0 {
		t.Errorf("%d bubbles after it left the top", n)
	}
}

// aboveAStalk reports whether a bubble is where one starts from a stalk of
// algae: on the row above its top, over a column it takes up.
func aboveAStalk(a *anim.Animation, b *anim.Entity) bool {
	for _, stalk := range a.EntitiesOfType("algae") {
		if b.Y == stalk.Y-1 && b.X >= stalk.X && b.X < stalk.X+float64(stalk.Width()) {
			return true
		}
	}
	return false
}

func TestThereAreAlwaysBubblesAtNight(t *testing.T) {
	const (
		pictures = 600
		warmUp   = 25 // pictures before the first bubbles have had time to rise
	)
	for seed := uint64(1); seed <= 5; seed++ {
		q, a := newTank(nightWidth, nightHeight, seed)
		q.SetNight(true)
		var empty, total int
		for picture := range pictures {
			q.NightBubble()
			for range NightStepsPerUpdate {
				a.Animate()
			}
			if picture < warmUp {
				continue
			}
			n := len(a.EntitiesOfType("bubble"))
			total += n
			if n == 0 {
				empty++
			}
		}
		if share := float64(empty) / (pictures - warmUp); share > maxEmptyShare {
			t.Errorf("seed %d: %.1f%% of the night's pictures have no bubble", seed, share*percent)
		}
		if mean := float64(total) / (pictures - warmUp); mean < minNightBubbles {
			t.Errorf("seed %d: %.1f bubbles in a picture of the night on average, want a few", seed, mean)
		}
	}
}

func TestTheAlgaeSwaysInEveryPictureOfTheNight(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		q, a := newTestAquarium(nightWidth, nightHeight, Options{}, seed)
		q.SetNight(true)
		q.Populate()
		picture := func() map[[2]int]rune {
			r := &recorder{t: t, w: nightWidth, h: nightHeight}
			a.Draw(r)
			return r.cells
		}
		last := picture()
		different, total := 0, 100
		for range total {
			for range NightStepsPerUpdate {
				a.Animate()
			}
			now := picture()
			if !sameCells(now, last) {
				different++
			}
			last = now
		}
		if got := float64(different) / float64(total); got < .75 {
			t.Errorf("seed %d: only %.0f%% of the night's pictures differ from the one before", seed, got*percent)
		}
	}
}

func sameCells(a, b map[[2]int]rune) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
