// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"testing"

	"go-asciiquarium/internal/anim"
)

// What a steady stream of bubbles is, for these tests.
const (
	maxEmptyShare     = .03 // at most this share of the pictures may have no bubble
	minDayBubbles     = 2   // bubbles in the water by day, on average
	minNightBubbles   = 3   // and in a picture of the night
	minFromAlgaeShare = .8  // most of the day's bubbles come from the algae
	rateTolerance     = 1.5 // percentage points around algaeBubblePercent
	minBubblesChecked = 10
)

// follow runs the tank for steps animation steps and tells, for each bubble
// that appears, where it started.
func follow(a *anim.Animation, steps int, born func(b *anim.Entity)) {
	seen := map[*anim.Entity]bool{}
	for range steps {
		a.Animate()
		for _, b := range a.EntitiesOfType("bubble") {
			if !seen[b] {
				seen[b] = true
				born(b) // it has not moved yet: it was made during this step
			}
		}
	}
}

func TestTheDayAlwaysHasBubblesRisingFromTheAlgae(t *testing.T) {
	const (
		steps  = 3000
		warmUp = 50 // steps before the first bubbles have had time to rise
	)
	for seed := uint64(1); seed <= 5; seed++ {
		q, a := newTank(nightWidth, nightHeight, seed)
		_ = q
		var empty, total int
		for step := range steps {
			a.Animate()
			if step < warmUp {
				continue
			}
			n := len(a.EntitiesOfType("bubble"))
			total += n
			if n == 0 {
				empty++
			}
		}
		if share := float64(empty) / (steps - warmUp); share > maxEmptyShare {
			t.Errorf("seed %d: %.1f%% of the steps have no bubble in the water", seed, share*percent)
		}
		if mean := float64(total) / (steps - warmUp); mean < minDayBubbles {
			t.Errorf("seed %d: %.1f bubbles in the water on average, want a steady few", seed, mean)
		}
	}
}

func TestMostBubblesOfTheDayStartAboveAStalk(t *testing.T) {
	q, a := newTank(nightWidth, nightHeight, 4)
	_ = q
	var fromAlgae, other int
	follow(a, 4000, func(b *anim.Entity) {
		if aboveAStalk(a, b) {
			fromAlgae++
		} else {
			other++ // the fish blows some too
		}
	})
	if fromAlgae == 0 {
		t.Fatal("no bubble came from the algae")
	}
	if share := float64(fromAlgae) / float64(fromAlgae+other); share < minFromAlgaeShare {
		t.Errorf("%.0f%% of the bubbles came from the algae, the rest from the fish: want most", share*percent)
	}
}

func TestAStalkReleasesBubblesAtItsOwnRate(t *testing.T) {
	// Algae alone, no fish and no surface: every bubble is a stalk's.
	q, a := newTestAquarium(nightWidth, nightHeight, Options{}, 6)
	q.addAllSeaweed()
	stalks := len(a.EntitiesOfType("algae"))
	const steps = 20000
	var born int
	follow(a, steps, func(*anim.Entity) { born++ })
	got := float64(born) / float64(steps*stalks) * percent
	if got < algaeBubblePercent-rateTolerance || got > algaeBubblePercent+rateTolerance {
		t.Errorf("a stalk released a bubble in %.1f%% of the steps, want about %d%%", got, algaeBubblePercent)
	}
}

func TestBubblesRiseStraightUp(t *testing.T) {
	for _, night := range []bool{false, true} {
		q, a := newTank(nightWidth, nightHeight, 7)
		q.SetNight(night)
		type start struct{ x, y float64 }
		first := map[*anim.Entity]start{}
		last := map[*anim.Entity]float64{}
		check := func() {
			for _, b := range a.EntitiesOfType("bubble") {
				if s, ok := first[b]; !ok {
					first[b], last[b] = start{b.X, b.Y}, b.Y
				} else if b.X != s.x {
					t.Fatalf("night %v: a bubble moved sideways, from x=%v to x=%v", night, s.x, b.X)
				} else if b.Y > last[b] {
					t.Fatalf("night %v: a bubble sank, from y=%v to y=%v", night, last[b], b.Y)
				} else {
					last[b] = b.Y
				}
			}
		}
		for range 1500 {
			if night {
				q.NightBubble()
			}
			a.Animate()
			check()
		}
		if len(first) < minBubblesChecked {
			t.Errorf("night %v: only %d bubbles to check", night, len(first))
		}
	}
}

func TestABubbleIsFullSizeWhenItGetsToTheTop(t *testing.T) {
	last := float64(len(bubbleFrames) - 1)
	for _, stepsPerRow := range []int{1, NightStepsPerUpdate} {
		// from the fourth row on, as the stalks and the fish make them: it is
		// from there that there are frames enough to grow through
		for y := float64(len(bubbleFrames) - 1); y < 40; y++ {
			atTheTop := bubbleGrowth(y, stepsPerRow) * y * float64(stepsPerRow)
			if atTheTop < last || atTheTop >= last+1 {
				t.Errorf("rising %d steps a row from y=%v: frame %.2f at the top, want the last, %v", stepsPerRow, y, atTheTop, last)
			}
		}
	}
}
