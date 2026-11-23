// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"testing"

	"go-asciiquarium/internal/anim"
)

func newLow(w, h int, seed uint64) (*Aquarium, *anim.Animation) {
	q, a := newTestAquarium(w, h, Options{Low: true}, seed)
	q.Populate()
	return q, a
}

func theFish(t *testing.T, a *anim.Animation) *anim.Entity {
	t.Helper()
	fish := a.EntitiesOfType("fish")
	if len(fish) != 1 {
		t.Fatalf("%d fish, want 1", len(fish))
	}
	return fish[0]
}

func runUntilNoFood(a *anim.Animation, frames int) int {
	for i := 0; i < frames; i++ {
		if len(a.EntitiesOfType("food")) == 0 {
			return i
		}
		a.Animate()
	}
	return -1
}

func TestMirror(t *testing.T) {
	if got, want := lowFishArt[0], "<º(((((()=<"; got != want {
		t.Errorf("left fish = %q", got)
	}
	if got, want := lowFishArt[1], ">=())))))º>"; got != want {
		t.Errorf("right fish = %q, want %q", got, want)
	}
	for _, s := range []string{lowFishArt[0], "<[x]{y}/\\>", "a"} {
		if got := mirror(mirror(s)); got != s {
			t.Errorf("mirroring %q twice gives %q", s, got)
		}
	}
	if mirror("<[{(/") != `\)}]>` {
		t.Errorf("mirror(\"<[{(/\") = %q", mirror("<[{(/"))
	}
}

func TestLowPopulate(t *testing.T) {
	_, a := newLow(80, 24, 1)
	// The surface, one fish and 80/15 = 5 algae: nothing else.
	if n := a.EntityCount(); n != 1+1+5 {
		t.Errorf("%d entities, want 7", n)
	}
	if len(a.EntitiesOfType("waterline")) != 1 || len(a.EntitiesOfType("fish")) != 1 {
		t.Error("expected one water line and one fish")
	}
	f := theFish(t, a)
	if f.Width() != 11 || f.Height() != 1 {
		t.Errorf("fish is %dx%d, want 11x1", f.Width(), f.Height())
	}

	// The surface is a single line of '~' across the top row, and nothing
	// else is up there.
	r := &recorder{t: t, w: 80, h: 24}
	a.Draw(r)
	for x := 0; x < 80; x++ {
		if ch := r.cells[[2]int{x, 0}]; ch != '~' {
			t.Fatalf("top row has %q at x=%d", ch, x)
		}
	}
	for pos := range r.cells {
		if pos[1] == 1 && r.cells[pos] == '~' {
			t.Fatalf("a second water line at %v", pos)
		}
	}
}

func TestLowRebuildAfterRemoveAll(t *testing.T) {
	q, a := newLow(80, 24, 1)
	a.RemoveAll()
	a.Resize(100, 30)
	q.Populate()
	if n := a.EntityCount(); n != 1+1+100/15 {
		t.Errorf("%d entities, want %d", n, 1+1+100/15)
	}
}

func TestLowFishSwimsEverywhereAndTurnsAround(t *testing.T) {
	const w, h = 80, 24
	_, a := newLow(w, h, 3)
	fish := theFish(t, a)

	minX, maxX, minY, maxY := 1e9, -1e9, 1e9, -1e9
	flips, faced := 0, map[int]bool{}
	last, matching, moving := fish.Frame(), 0, 0
	lastX := fish.X
	for range 8000 {
		a.Animate()
		if fish.X < 0 || fish.X > w-11 || fish.Y < 1 || fish.Y > h-1 {
			t.Fatalf("fish out of the water at (%v,%v)", fish.X, fish.Y)
		}
		minX, maxX = min(minX, fish.X), max(maxX, fish.X)
		minY, maxY = min(minY, fish.Y), max(maxY, fish.Y)
		if f := fish.Frame(); f != last {
			flips++
			last = f
		}
		faced[fish.Frame()] = true

		// Frame 1 faces right: it should be swimming that way.
		if dx := fish.X - lastX; dx > .05 || dx < -.05 {
			moving++
			if (dx > 0) == (fish.Frame() == 1) {
				matching++
			}
		}
		lastX = fish.X
	}

	if maxX-minX < 50 || maxY-minY < 15 {
		t.Errorf("fish only covered x %v..%v and y %v..%v", minX, maxX, minY, maxY)
	}
	if !faced[0] || !faced[1] || flips < 20 {
		t.Errorf("fish faced %v and turned %d times", faced, flips)
	}
	if moving == 0 || float64(matching)/float64(moving) < .97 {
		t.Errorf("fish faced the way it swam in %d of %d frames", matching, moving)
	}
}

func TestLowFishSpriteIsMirrored(t *testing.T) {
	_, a := newLow(80, 24, 3)
	fish := theFish(t, a)
	fish.X, fish.Y = 10, 12
	for frame, want := range lowFishArt {
		fish.SetFrame(frame)
		r := &recorder{t: t, w: 80, h: 24}
		a.Draw(r)
		var got []rune
		for x := 10; x < 21; x++ {
			got = append(got, r.cells[[2]int{x, 12}])
		}
		if string(got) != want {
			t.Errorf("frame %d draws %q, want %q", frame, string(got), want)
		}
	}
}

func TestBubblesRiseToTheSurfaceAndPop(t *testing.T) {
	_, a := newLow(80, 24, 5)
	seen, most := 0, 0
	for range 4000 {
		a.Animate()
		bubbles := a.EntitiesOfType("bubble")
		seen += len(bubbles)
		most = max(most, len(bubbles))
		for _, b := range bubbles {
			// They pop on the surface row instead of floating off the screen.
			if b.Y < 0 {
				t.Fatalf("a bubble got above the surface: y=%v", b.Y)
			}
		}
	}
	if seen == 0 {
		t.Error("the fish never blew a bubble")
	}
	if most > 24 {
		t.Errorf("%d bubbles at once", most)
	}
}

func TestDropFood(t *testing.T) {
	q, a := newLow(100, 30, 1)
	counts := map[int]bool{}
	velocities := map[float64]bool{}
	for range 300 {
		a.RemoveAll()
		q.DropFood()
		counts[a.EntityCount()] = true
		for _, f := range a.EntitiesOfType("food") {
			// From the middle, spread out by up to 100/8 = 12 cells.
			if f.X < 50-12 || f.X > 50+12 {
				t.Fatalf("pellet at x=%v is too far from the middle", f.X)
			}
			if f.Y != 1 {
				t.Fatalf("pellet starts at y=%v, want the top (1)", f.Y)
			}
			if f.Delta.Y < .1 || f.Delta.Y >= .3 {
				t.Fatalf("pellet speed %v not in 0.1..0.3", f.Delta.Y)
			}
			velocities[f.Delta.Y] = true
		}
	}
	for n := 5; n <= 10; n++ {
		if !counts[n] {
			t.Errorf("never dropped %d pellets", n)
		}
	}
	if len(counts) != 6 {
		t.Errorf("pellet counts seen: %v, want exactly 5..10", counts)
	}
	if len(velocities) < 1000 {
		t.Errorf("only %d different speeds in ~2000 pellets", len(velocities))
	}
}

func TestDropFoodSpreadsOverTheMiddleOfTheTank(t *testing.T) {
	q, a := newLow(200, 30, 1)
	a.RemoveAll()
	minX, maxX := 1e9, -1e9
	for range 50 {
		q.DropFood()
	}
	for _, f := range a.EntitiesOfType("food") {
		minX, maxX = min(minX, f.X), max(maxX, f.X)
	}
	// A spread of width/8 = 25 on both sides of x=100, and it uses most of it.
	if minX < 75 || maxX > 125 || maxX-minX < 35 {
		t.Errorf("pellets between x=%v and x=%v", minX, maxX)
	}
}

func TestDropFoodDoesNothingInTheNormalAquarium(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	q.Populate()
	before := a.EntityCount()
	q.DropFood()
	if a.EntityCount() != before || len(a.EntitiesOfType("food")) != 0 {
		t.Error("food was dropped in the normal aquarium")
	}
}

func TestFoodSinksSlowlyAndStaysOnTheBottom(t *testing.T) {
	const w, h = 80, 24
	q, a := newTestAquarium(w, h, Options{Low: true}, 2) // no fish to eat it
	q.DropFood()
	food := a.EntitiesOfType("food")
	if len(food) < 5 {
		t.Fatalf("%d pellets", len(food))
	}

	for range 10 {
		a.Animate()
	}
	ys := map[float64]bool{}
	for _, f := range food {
		ys[f.Y] = true
		// 10 frames at under 0.3 rows each: still near the top.
		if f.Y <= 1 || f.Y > 1+10*.3 {
			t.Errorf("after 10 frames a pellet is at y=%v", f.Y)
		}
	}
	if len(ys) < 3 {
		t.Errorf("pellets sink in step: %d different heights", len(ys))
	}

	// The slowest one takes under 24/0.1 frames to land.
	for range 300 {
		a.Animate()
	}
	if got := len(a.EntitiesOfType("food")); got != len(food) {
		t.Errorf("%d pellets left of %d: they must stay", got, len(food))
	}
	r := &recorder{t: t, w: w, h: h}
	a.Draw(r)
	for _, f := range food {
		if f.Y != h-1 {
			t.Errorf("pellet at y=%v, want the bottom row %d", f.Y, h-1)
		}
		if r.cells[[2]int{int(f.X), h - 1}] != '*' {
			t.Errorf("no '*' drawn at (%v,%d)", f.X, h-1)
		}
	}
}

func TestFoodIsCapped(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{Low: true}, 2)
	for range 40 {
		q.DropFood()
	}
	if n := len(a.EntitiesOfType("food")); n != maxFood {
		t.Errorf("%d pellets, want %d", n, maxFood)
	}
}

func TestFishGoesForTheClosestPellet(t *testing.T) {
	q, a := newLow(120, 24, 7)
	fish := theFish(t, a)
	fish.X, fish.Y = 10, 10
	// None of them lies on the way to another one: swimming towards the
	// farthest (x=100, same row as the fish) would not pass the others.
	q.addFood(100, 10, 0)
	q.addFood(60, 4, 0)
	q.addFood(20, 18, 0) // the closest

	pellets := a.EntitiesOfType("food")
	var order []float64
	eaten := map[*anim.Entity]bool{}
	for i := 0; i < 600 && len(order) < 3; i++ {
		a.Animate()
		alive := map[*anim.Entity]bool{}
		for _, p := range a.EntitiesOfType("food") {
			alive[p] = true
		}
		for _, p := range pellets {
			if !alive[p] && !eaten[p] {
				eaten[p] = true
				order = append(order, p.X)
			}
		}
	}
	if fmt.Sprint(order) != "[20 60 100]" {
		t.Errorf("pellets eaten in the order of x = %v, want [20 60 100]", order)
	}
}

func TestFishTurnsAndSwimsTowardsFood(t *testing.T) {
	q, a := newLow(120, 24, 7)
	fish := theFish(t, a)
	for _, side := range []struct {
		fishX, foodX float64
		right        bool
	}{{10, 100, true}, {100, 10, false}} {
		fish.X, fish.Y = side.fishX, 12
		q.addFood(side.foodX, 12, 0)
		start := fish.X
		for range 5 {
			a.Animate()
		}
		if got := fish.Frame() == 1; got != side.right {
			t.Errorf("food at x=%v, fish at x=%v: facing right = %v", side.foodX, start, got)
		}
		if moved := fish.X - start; (moved > 0) != side.right || moved*moved < 9 {
			t.Errorf("fish moved %v towards food at x=%v", moved, side.foodX)
		}
		if runUntilNoFood(a, 500) < 0 {
			t.Fatal("the food was never eaten")
		}
	}
}

func TestFishEatsEverythingIncludingWhatLiesOnTheBottom(t *testing.T) {
	const w, h = 80, 24
	q, a := newLow(w, h, 4)
	fish := theFish(t, a)
	fish.X, fish.Y = 0, 1

	q.DropFood()
	n := len(a.EntitiesOfType("food"))
	q.addFood(5, h-1, 0) // already on the bottom, far from the fish
	q.addFood(w-1, h-1, 0)
	if frames := runUntilNoFood(a, 6000); frames < 0 {
		t.Fatalf("%d pellets left", len(a.EntitiesOfType("food")))
	}
	if len(a.EntitiesOfType("fish")) != 1 {
		t.Error("the fish is gone")
	}
	if n < 5 {
		t.Errorf("only %d pellets were dropped", n)
	}
}

func TestFishKeepsSwimmingWhenTheFoodIsGone(t *testing.T) {
	q, a := newLow(80, 24, 4)
	fish := theFish(t, a)
	q.DropFood()
	if runUntilNoFood(a, 6000) < 0 {
		t.Fatal("food not eaten")
	}
	x, y := fish.X, fish.Y
	for range 200 {
		a.Animate()
	}
	if fish.X == x && fish.Y == y {
		t.Error("the fish stopped moving")
	}
}

func TestLowSimulation(t *testing.T) {
	sizes := [][2]int{{80, 24}, {200, 60}, {40, 12}, {20, 6}, {12, 3}, {5, 2}}
	frames := 6000
	if testing.Short() {
		frames = 1500
	}
	for _, size := range sizes {
		for seed := uint64(1); seed <= 3; seed++ {
			t.Run(fmt.Sprintf("%dx%d/seed=%d", size[0], size[1], seed), func(t *testing.T) {
				w, h := size[0], size[1]
				q, a := newLow(w, h, seed)
				canvas := &recorder{t: t, w: w, h: h}
				floor := 2 + w/15 // surface, fish, algae
				for i := range frames {
					if i%150 == 0 {
						q.DropFood()
					}
					a.Animate()
					a.Draw(canvas)
					if n := a.EntityCount(); n < floor {
						t.Fatalf("frame %d: %d entities, want at least %d", i, n, floor)
					}
				}
			})
		}
	}
}
