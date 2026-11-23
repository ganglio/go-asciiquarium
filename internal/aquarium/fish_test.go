// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"testing"

	"go-asciiquarium/internal/anim"
	"go-asciiquarium/internal/flip"
)

// fishWidth is how wide the fish is, whatever it looks like.
var fishWidth = len([]rune(fishArt[frameLeft]))

func runUntilNoFood(a *anim.Animation, steps int) int {
	for i := 0; i < steps; i++ {
		if len(a.EntitiesOfType("food")) == 0 {
			return i
		}
		a.Animate()
	}
	return -1
}

func TestFishFacesBothWays(t *testing.T) {
	// The second sprite is the first one flipped, and just as wide.
	left, right := fishArt[frameLeft], fishArt[frameRight]
	if right != flip.String(left) || flip.String(right) != left {
		t.Errorf("%q is not the mirror image of %q", right, left)
	}
	if left == right {
		t.Error("the fish looks the same facing both ways")
	}
	if w := len([]rune(right)); w != fishWidth {
		t.Errorf("the sprites are %d and %d characters wide", fishWidth, w)
	}
	if left != fishFacingLeft {
		t.Errorf("the left sprite is %q, not the fish %q", left, fishFacingLeft)
	}
}

func TestFishSwimsEverywhereAndTurnsAround(t *testing.T) {
	for _, aspect := range []float64{0, badgeAspect} {
		t.Run(fmt.Sprintf("aspect=%.2f", aspect), func(t *testing.T) {
			const w, h = 80.0, 24.0
			q, a := newTestAquarium(int(w), int(h), Options{CellAspect: aspect}, 3)
			q.Populate()
			fish := theFish(t, a)

			minX, maxX, minY, maxY := 1e9, -1e9, 1e9, -1e9
			flips, faced := 0, map[int]bool{}
			last, matching, moving := fish.Frame(), 0, 0
			lastX := fish.X
			for range 8000 {
				a.Animate()
				if fish.X < 0 || fish.X > w-float64(fishWidth) || fish.Y < 1 || fish.Y > h-1 {
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
					if (dx > 0) == (fish.Frame() == frameRight) {
						matching++
					}
				}
				lastX = fish.X
			}

			// It should use most of the water: at least two thirds of its width and
			// of its depth.
			const most = 2.0 / 3
			if maxX-minX < (w-float64(fishWidth))*most || maxY-minY < (h-firstWaterRow-1)*most {
				t.Errorf("fish only covered x %v..%v and y %v..%v", minX, maxX, minY, maxY)
			}
			if !faced[0] || !faced[1] || flips < 20 {
				t.Errorf("fish faced %v and turned %d times", faced, flips)
			}
			if moving == 0 || float64(matching)/float64(moving) < .97 {
				t.Errorf("fish faced the way it swam in %d of %d steps", matching, moving)
			}
		})
	}
}

func TestFishSpriteIsMirrored(t *testing.T) {
	_, a := newTank(80, 24, 3)
	fish := theFish(t, a)
	fish.X, fish.Y = 10, 12
	for frame, want := range fishArt {
		fish.SetFrame(frame)
		r := &recorder{t: t, w: 80, h: 24}
		a.Draw(r)
		var got []rune
		for x := 10; x < 10+fishWidth; x++ {
			got = append(got, r.cells[[2]int{x, 12}])
		}
		if string(got) != want {
			t.Errorf("frame %d draws %q, want %q", frame, string(got), want)
		}
	}
}

func TestDropFood(t *testing.T) {
	const tankWidth = 100
	q, a := newTank(tankWidth, 30, 1)
	counts := map[int]bool{}
	velocities := map[float64]bool{}
	for range 300 {
		a.RemoveAll()
		q.DropFood()
		counts[a.EntityCount()] = true
		if q.FoodCount() != a.EntityCount() {
			t.Fatalf("FoodCount() = %d, entities = %d", q.FoodCount(), a.EntityCount())
		}
		for _, f := range a.EntitiesOfType("food") {
			// From the middle of the tank, spread out by the usual amount.
			middle, spread := float64(tankWidth/2), float64(max(minSpread, tankWidth/spreadDivisor))
			if f.X < middle-spread || f.X > middle+spread {
				t.Fatalf("pellet at x=%v is too far from the middle", f.X)
			}
			if f.Y != firstWaterRow {
				t.Fatalf("pellet starts at y=%v, want the top (%d)", f.Y, firstWaterRow)
			}
			if f.Delta.Y < minFall || f.Delta.Y >= minFall+extraFall {
				t.Fatalf("pellet speed %v not in %v..%v", f.Delta.Y, minFall, minFall+extraFall)
			}
			velocities[f.Delta.Y] = true
		}
	}
	for n := minPellets; n <= maxPellets; n++ {
		if !counts[n] {
			t.Errorf("never dropped %d pellets", n)
		}
	}
	if want := maxPellets - minPellets + 1; len(counts) != want {
		t.Errorf("pellet counts seen: %v, want exactly %d..%d", counts, minPellets, maxPellets)
	}
	if len(velocities) < 1000 {
		t.Errorf("only %d different speeds in ~2000 pellets", len(velocities))
	}
}

func TestDropFoodSpreadsOverTheMiddleOfTheTank(t *testing.T) {
	q, a := newTank(200, 30, 1)
	a.RemoveAll()
	minX, maxX := 1e9, -1e9
	for range 50 {
		q.DropFood()
	}
	for _, f := range a.EntitiesOfType("food") {
		minX, maxX = min(minX, f.X), max(maxX, f.X)
	}
	// A spread of a spreadDivisor-th of the width on both sides of the middle,
	// and it uses most of it.
	const width = 200
	middle, spread := float64(width/2), float64(width/spreadDivisor)
	if minX < middle-spread || maxX > middle+spread || maxX-minX < spread {
		t.Errorf("pellets between x=%v and x=%v, the spread is %v around %v", minX, maxX, spread, middle)
	}
}

func TestFoodSinksSlowlyAndStaysOnTheBottom(t *testing.T) {
	const w, h = 80, 24
	q, a := newTestAquarium(w, h, Options{}, 2) // no fish to eat it
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
		// 10 steps at under the top speed: still near the top.
		if f.Y <= firstWaterRow || f.Y > firstWaterRow+10*(minFall+extraFall) {
			t.Errorf("after 10 steps a pellet is at y=%v", f.Y)
		}
	}
	if len(ys) < 3 {
		t.Errorf("pellets sink in step: %d different heights", len(ys))
	}

	// The slowest one takes under 24/0.1 steps to land.
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
	q, a := newTestAquarium(80, 24, Options{}, 2)
	for range 40 {
		q.DropFood()
	}
	if n := len(a.EntitiesOfType("food")); n != maxFood {
		t.Errorf("%d pellets, want %d", n, maxFood)
	}
}

func TestFishGoesForTheClosestPellet(t *testing.T) {
	q, a := newTank(120, 24, 7)
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
	q, a := newTank(120, 24, 7)
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
		if got := fish.Frame() == frameRight; got != side.right {
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
	q, a := newTank(w, h, 4)
	fish := theFish(t, a)
	fish.X, fish.Y = 0, 1

	q.DropFood()
	n := q.FoodCount()
	q.addFood(5, h-1, 0) // already on the bottom, far from the fish
	q.addFood(w-1, h-1, 0)
	if steps := runUntilNoFood(a, 6000); steps < 0 {
		t.Fatalf("%d pellets left", q.FoodCount())
	}
	if len(a.EntitiesOfType("fish")) != 1 {
		t.Error("the fish is gone")
	}
	if n < 5 {
		t.Errorf("only %d pellets were dropped", n)
	}
}

func TestFishKeepsSwimmingWhenTheFoodIsGone(t *testing.T) {
	q, a := newTank(80, 24, 4)
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
