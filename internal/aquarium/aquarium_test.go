// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"go-asciiquarium/internal/anim"
)

// badgeAspect is the shape of a cell of the e-ink screen: 6 pixels wide and
// 8 tall.
const badgeAspect = 8.0 / 6.0

func newTestAquarium(w, h int, opts Options, seed uint64) (*Aquarium, *anim.Animation) {
	a := anim.New(w, h)
	return New(a, opts, rand.New(rand.NewPCG(seed, seed*7+1))), a
}

func newTank(w, h int, seed uint64) (*Aquarium, *anim.Animation) {
	q, a := newTestAquarium(w, h, Options{}, seed)
	q.Populate()
	return q, a
}

// recorder is a Canvas that remembers the cells drawn and complains about
// cells outside the grid.
type recorder struct {
	t     *testing.T
	w, h  int
	cells map[[2]int]rune
}

func (r *recorder) Clear() { r.cells = map[[2]int]rune{} }

func (r *recorder) SetCell(x, y int, ch rune) {
	if x < 0 || y < 0 || x >= r.w || y >= r.h {
		r.t.Fatalf("cell (%d,%d) drawn outside %dx%d", x, y, r.w, r.h)
	}
	r.cells[[2]int{x, y}] = ch
}

func theFish(t *testing.T, a *anim.Animation) *anim.Entity {
	t.Helper()
	fish := a.EntitiesOfType("fish")
	if len(fish) != 1 {
		t.Fatalf("%d fish, want 1", len(fish))
	}
	return fish[0]
}

func TestPopulate(t *testing.T) {
	const width, height = 80, 24
	_, a := newTank(width, height, 1)
	// The surface, one fish and a stalk of algae for each algaeSpacing columns:
	// nothing else.
	want := 1 + 1 + width/algaeSpacing
	if n := a.EntityCount(); n != want {
		t.Errorf("%d entities, want %d", n, want)
	}
	if len(a.EntitiesOfType("waterline")) != 1 || len(a.EntitiesOfType("fish")) != 1 {
		t.Error("expected one water line and one fish")
	}
	f := theFish(t, a)
	if f.Width() != fishWidth || f.Height() != 1 {
		t.Errorf("fish is %dx%d, want %dx1", f.Width(), f.Height(), fishWidth)
	}

	// The surface is a single line of '~' across the top row, and nothing
	// else is up there.
	r := &recorder{t: t, w: width, h: height}
	a.Draw(r)
	for x := 0; x < width; x++ {
		if ch := r.cells[[2]int{x, surfaceRow}]; ch != '~' {
			t.Fatalf("top row has %q at x=%d", ch, x)
		}
	}
	for pos, ch := range r.cells {
		if pos[1] == firstWaterRow && ch == '~' {
			t.Fatalf("a second water line at %v", pos)
		}
	}
}

func TestRebuildAfterRemoveAll(t *testing.T) {
	q, a := newTank(80, 24, 1)
	a.RemoveAll()
	a.Resize(100, 30)
	q.Populate()
	want := 1 + 1 + 100/algaeSpacing
	if n := a.EntityCount(); n != want {
		t.Errorf("%d entities, want %d", n, want)
	}
}

func TestBubblePopsAtTheSurface(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	q.addSurface()
	fish := anim.NewEntity(anim.Config{Shape: []string{"abc\nabc\nabc"}, X: 10, Y: 12, Z: 5})
	q.addBubble(fish, true)

	bubbles := a.EntitiesOfType("bubble")
	if len(bubbles) != 1 {
		t.Fatalf("%d bubbles", len(bubbles))
	}
	b := bubbles[0]
	// A right-facing fish blows its bubbles at its right edge, half way down,
	// and one layer in front of itself.
	if b.X != 13 || b.Y != 13 || b.Z != 4 {
		t.Errorf("bubble starts at (%v,%v,%v), want (13,13,4)", b.X, b.Y, b.Z)
	}

	// It rises one row per step and pops on reaching the surface row (y=0),
	// instead of floating off the grid.
	for i := 1; i <= 12; i++ {
		a.Animate()
		if len(a.EntitiesOfType("bubble")) != 1 {
			t.Fatalf("bubble gone after %d steps", i)
		}
	}
	a.Animate()
	if len(a.EntitiesOfType("bubble")) != 0 {
		t.Errorf("bubble survived reaching the surface (y=%v)", b.Y)
	}
}

func TestBubbleFromLeftFacingFish(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	fish := anim.NewEntity(anim.Config{Shape: []string{"abc"}, X: 10, Y: 12, Z: 5})
	q.addBubble(fish, false)
	if b := a.EntitiesOfType("bubble")[0]; b.X != 10 {
		t.Errorf("bubble x = %v, want 10", b.X)
	}
}

func TestBubblesRiseToTheSurfaceAndPop(t *testing.T) {
	_, a := newTank(80, 24, 5)
	seen, most := 0, 0
	for range 4000 {
		a.Animate()
		bubbles := a.EntitiesOfType("bubble")
		seen += len(bubbles)
		most = max(most, len(bubbles))
		for _, b := range bubbles {
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

func TestSeaweedSways(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 9)
	q.addSeaweed()

	stalk := func() []rune {
		r := &recorder{t: t, w: 80, h: 24}
		a.Draw(r)
		top := 24
		for pos := range r.cells {
			top = min(top, pos[1])
		}
		var out []rune
		for y := top; y < 24; y++ {
			var found rune
			for pos, ch := range r.cells {
				if pos[1] == y {
					found = ch
				}
			}
			out = append(out, found)
		}
		return out
	}

	before := stalk()
	minHeight, maxHeight := algaeMinHeight, algaeMinHeight+algaeExtraHeight-1
	if n := len(before); n < minHeight || n > maxHeight {
		t.Fatalf("seaweed is %d rows high, want %d-%d", n, minHeight, maxHeight)
	}
	// The stalk alternates between ')' and '(' going down, and the bottom row
	// touches the bottom of the grid.
	for i := 1; i < len(before); i++ {
		if before[i] == before[i-1] {
			t.Errorf("stalk %q does not alternate", string(before))
		}
	}
	for range 4 { // at least four steps per sway
		a.Animate()
	}
	after := stalk()
	for i := range before {
		if before[i] == after[i] {
			t.Errorf("stalk did not sway: %q -> %q", string(before), string(after))
			break
		}
	}
}

func TestSeaweedIsReplacedWhenItDies(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	q, a := newTestAquarium(80, 24, Options{}, 2)
	a.Now = func() time.Time { return now }
	q.Populate()
	before := a.EntityCount()
	now = now.Add(algaeMinLife + algaeExtraLife + time.Minute) // longer than any stalk lives
	a.Animate()
	// Each stalk is replaced by a new one, so there are as many as before
	// (plus perhaps a bubble) and none is left to expire right away.
	if after := a.EntityCount(); after < before || after > before+1 {
		t.Errorf("entity count went from %d to %d: seaweed was not replaced", before, after)
	}
	a.Animate()
	if a.EntityCount() < before {
		t.Errorf("entities vanished: %d -> %d", before, a.EntityCount())
	}
}

// TestSimulation runs the whole aquarium for a long time on grids of many
// sizes, including absurdly small ones, with food dropped now and then, and
// checks that nothing panics, nothing is drawn outside the grid and nothing
// that should stay disappears.
func TestSimulation(t *testing.T) {
	sizes := [][2]int{{44, 22}, {80, 24}, {200, 60}, {40, 12}, {20, 6}, {12, 3}, {5, 2}}
	steps := 6000
	if testing.Short() {
		steps = 1500
	}
	for _, size := range sizes {
		for _, aspect := range []float64{0, badgeAspect} {
			for seed := uint64(1); seed <= 3; seed++ {
				name := fmt.Sprintf("%dx%d/aspect=%.2f/seed=%d", size[0], size[1], aspect, seed)
				t.Run(name, func(t *testing.T) {
					w, h := size[0], size[1]
					q, a := newTestAquarium(w, h, Options{CellAspect: aspect}, seed)
					q.Populate()
					canvas := &recorder{t: t, w: w, h: h}
					floor := 2 + w/algaeSpacing // surface, fish, algae
					for i := range steps {
						if i%150 == 0 {
							q.DropFood()
						}
						a.Animate()
						a.Draw(canvas)
						if n := a.EntityCount(); n < floor || n > floor+400 {
							t.Fatalf("step %d: %d entities, want %d..%d", i, n, floor, floor+400)
						}
					}
				})
			}
		}
	}
}
