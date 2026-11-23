// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"go-asciiquarium/internal/anim"
)

func newTestAquarium(w, h int, opts Options, seed uint64) (*Aquarium, *anim.Animation) {
	a := anim.New(w, h)
	return New(a, opts, rand.New(rand.NewPCG(seed, seed*7+1))), a
}

// recorder is a Canvas that remembers the cells drawn and complains about
// cells outside the screen.
type recorder struct {
	t     *testing.T
	w, h  int
	cells map[[2]int]rune
	count int
}

func (r *recorder) Clear() { r.cells = map[[2]int]rune{} }

func (r *recorder) SetCell(x, y int, ch rune, _ anim.Color) {
	if x < 0 || y < 0 || x >= r.w || y >= r.h {
		r.t.Fatalf("cell (%d,%d) drawn outside %dx%d", x, y, r.w, r.h)
	}
	r.cells[[2]int{x, y}] = ch
	r.count++
}

func TestPopulate(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	q.Populate()

	if n := len(a.EntitiesOfType("waterline")); n != 4 {
		t.Errorf("%d water lines, want 4", n)
	}
	// One fish per 350 cells of water: (24-9)*80/350 = 3.
	if n := len(a.EntitiesOfType("fish")); n != 3 {
		t.Errorf("%d fish, want 3", n)
	}
	// 4 water lines + castle + 80/15 = 5 seaweed + 3 fish + 1 random creature;
	// a shark brings its teeth along.
	got := a.EntityCount()
	want := 4 + 1 + 5 + 3 + 1
	if teeth := len(a.EntitiesOfType("teeth")); teeth > 0 {
		want += teeth
	}
	if got != want {
		t.Errorf("%d entities, want %d", got, want)
	}
}

func TestPopulateScalesWithScreen(t *testing.T) {
	q, a := newTestAquarium(200, 60, Options{}, 1)
	q.Populate()
	// (60-9)*200/350 = 29 fish.
	if n := len(a.EntitiesOfType("fish")); n != 29 {
		t.Errorf("%d fish, want 29", n)
	}
}

func TestRebuildAfterRemoveAll(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	q.Populate()
	a.RemoveAll()
	if a.EntityCount() != 0 {
		t.Fatal("RemoveAll left entities behind")
	}
	a.Resize(100, 30)
	q.Populate()
	// (30-9)*100/350 = 6 fish, and water lines tiled over the new width.
	if n := len(a.EntitiesOfType("fish")); n != 6 {
		t.Errorf("%d fish, want 6", n)
	}
	for _, w := range a.EntitiesOfType("waterline") {
		if w.Width() < 100 {
			t.Errorf("water line is %d wide, want at least 100", w.Width())
		}
	}
}

func TestSharkEatsSmallFishButNotBigOnes(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 3)
	q.addShark()
	teeth := a.EntitiesOfType("teeth")
	if len(teeth) != 1 {
		t.Fatalf("%d teeth", len(teeth))
	}
	// Put two stationary fish where the teeth will be after this frame.
	x, y := teeth[0].X+teeth[0].Delta.X, teeth[0].Y
	fish := func(shape string) {
		a.Spawn(anim.Config{
			Type: "fish", Shape: []string{shape}, X: x, Y: y, Z: 5,
			Physical: true, CollisionHandler: q.fishCollision,
		})
	}
	fish("<><")                          // 1 row: eaten
	fish("<><\n<><\n<><\n<><\n<><")      // 5 rows: still small
	fish("<><\n<><\n<><\n<><\n<><\n<><") // 6 rows: safe

	a.Animate()
	var heights []int
	for _, f := range a.EntitiesOfType("fish") {
		heights = append(heights, f.Height())
	}
	if fmt.Sprint(heights) != "[6]" {
		t.Errorf("surviving fish heights = %v, want [6]", heights)
	}

	// shark + teeth + big fish + 2 splats; the splats disappear after 16 frames.
	if n := a.EntityCount(); n != 5 {
		t.Errorf("%d entities after the meal, want 5", n)
	}
	for range 15 {
		a.Animate()
	}
	if n := a.EntityCount(); n != 5 {
		t.Errorf("splats gone too early: %d entities", n)
	}
	a.Animate()
	if n := a.EntityCount(); n != 3 {
		t.Errorf("splats still there after 16 frames: %d entities", n)
	}
}

func TestFishAtOtherDepthsAreNotEaten(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 3)
	q.addShark()
	teeth := a.EntitiesOfType("teeth")[0]
	// The teeth cover depths 3..19; the water line layers are in front.
	a.Spawn(anim.Config{
		Type: "fish", X: teeth.X + teeth.Delta.X, Y: teeth.Y, Z: 20,
		Physical: true, CollisionHandler: q.fishCollision,
	})
	a.Animate()
	if n := len(a.EntitiesOfType("fish")); n != 1 {
		t.Errorf("fish at z=20 was eaten")
	}
}

func TestSharkDeathRemovesTeethAndSpawnsSomethingNew(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 5)
	q.addShark()
	shark := a.EntityCount() // shark + teeth
	if shark != 2 {
		t.Fatalf("%d entities", shark)
	}
	// The shark swims across the screen and leaves; by then something else
	// must have taken its place and the teeth must be gone.
	for range 200 {
		a.Animate()
		if len(a.EntitiesOfType("teeth")) == 0 {
			break
		}
	}
	if a.EntityCount() == 0 {
		t.Error("nothing replaced the shark")
	}
	// Teeth only ever exist together with a shark: no orphans.
	if n := len(a.EntitiesOfType("teeth")); n > 1 {
		t.Errorf("%d teeth", n)
	}
}

func TestBubblePopsAtTheWaterLine(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	q.addEnvironment()
	fish := anim.NewEntity(anim.Config{
		Shape: []string{"abc\nabc\nabc"}, X: 10, Y: 12, Z: 5, Delta: anim.Delta{X: 1},
	})
	q.addBubble(fish, true)

	bubbles := a.EntitiesOfType("bubble")
	if len(bubbles) != 1 {
		t.Fatalf("%d bubbles", len(bubbles))
	}
	b := bubbles[0]
	// A right-moving fish blows its bubbles at its right edge, half way down,
	// and one layer in front of itself.
	if b.X != 13 || b.Y != 13 || b.Z != 4 {
		t.Errorf("bubble starts at (%v,%v,%v), want (13,13,4)", b.X, b.Y, b.Z)
	}

	// It rises one row per frame and pops on reaching the lowest water line
	// row (y=8), long before it would leave the screen.
	for i := 1; i <= 4; i++ {
		a.Animate()
		if len(a.EntitiesOfType("bubble")) != 1 {
			t.Fatalf("bubble gone after %d frames", i)
		}
	}
	a.Animate()
	if len(a.EntitiesOfType("bubble")) != 0 {
		t.Errorf("bubble survived reaching the water line (y=%v)", b.Y)
	}
}

func TestBubbleFromLeftMovingFish(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 1)
	fish := anim.NewEntity(anim.Config{
		Shape: []string{"abc"}, X: 10, Y: 12, Z: 5, Delta: anim.Delta{X: -1},
	})
	q.addBubble(fish, false)
	if b := a.EntitiesOfType("bubble")[0]; b.X != 10 {
		t.Errorf("bubble x = %v, want 10", b.X)
	}
}

func TestSeaweedSways(t *testing.T) {
	q, a := newTestAquarium(80, 24, Options{}, 9)
	q.addSeaweed()

	rows := func() (stalk []rune, x int) {
		r := &recorder{t: t, w: 80, h: 24}
		a.Draw(r)
		top := 24
		for pos := range r.cells {
			top = min(top, pos[1])
		}
		x = -1
		for y := top; y < 24; y++ {
			for pos, ch := range r.cells {
				if pos[1] == y && ch == '(' {
					x = pos[0]
				}
			}
		}
		for y := top; y < 24; y++ {
			var found rune
			for pos, ch := range r.cells {
				if pos[1] == y {
					found = ch
				}
			}
			stalk = append(stalk, found)
		}
		return stalk, x
	}

	before, _ := rows()
	if n := len(before); n < 3 || n > 6 {
		t.Fatalf("seaweed is %d rows high, want 3-6", n)
	}
	// The stalk alternates between ')' and '(' going down, and the bottom row
	// touches the bottom of the screen.
	for i := 1; i < len(before); i++ {
		if before[i] == before[i-1] {
			t.Errorf("stalk %q does not alternate", string(before))
		}
	}
	for range 4 { // at least four frames per sway
		a.Animate()
	}
	after, _ := rows()
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
	now = now.Add(13 * time.Minute) // seaweed lives for 8 to 12 minutes
	a.Animate()
	if after := a.EntityCount(); after < before-1 || after > before+10 {
		t.Errorf("entity count went from %d to %d: seaweed was not replaced", before, after)
	}
	// Every seaweed was replaced by a new one, so there are still 5 stalks and
	// nothing was left to expire again right away.
	a.Animate()
	if a.EntityCount() < before-1 {
		t.Errorf("entities vanished: %d -> %d", before, a.EntityCount())
	}
}

func TestRandColor(t *testing.T) {
	q, _ := newTestAquarium(80, 24, Options{}, 4)
	const in = "123 456\n789 W"
	seen := map[rune]bool{}
	for range 2000 {
		out := q.randColor(in)
		if len(out) != len(in) {
			t.Fatalf("length changed: %q", out)
		}
		for i, ch := range out {
			switch c := in[i]; {
			case c >= '1' && c <= '9':
				seen[ch] = true
				if !strings.ContainsRune("cCrRyYbBgGm", ch) {
					t.Fatalf("digit %c became %q in %q", c, ch, out)
				}
			case ch != rune(c):
				t.Fatalf("%q changed to %q", c, ch)
			}
		}
	}
	// Every color of the palette shows up, except the last one ('M'), which
	// the original never picks.
	for _, c := range "cCrRyYbBgGm" {
		if !seen[c] {
			t.Errorf("color %c never chosen", c)
		}
	}
	if seen['M'] {
		t.Error("'M' chosen")
	}
}

func TestRandColorUsesOneColorPerDigit(t *testing.T) {
	q, _ := newTestAquarium(80, 24, Options{}, 4)
	out := q.randColor("1 1 1\n2 2 2")
	rows := strings.Split(out, "\n")
	for _, row := range rows {
		f := strings.Fields(row)
		if f[0] != f[1] || f[1] != f[2] {
			t.Errorf("digits in %q did not get the same color", row)
		}
	}
}

// TestSimulation runs the whole aquarium for a long time on screens of many
// sizes, including absurdly small ones, and checks that nothing panics,
// nothing is drawn off screen and the scene never runs out of creatures.
func TestSimulation(t *testing.T) {
	sizes := [][2]int{{80, 24}, {120, 40}, {200, 60}, {40, 12}, {30, 10}, {10, 5}, {5, 3}}
	frames := 12000 // about 20 minutes of aquarium time
	if testing.Short() {
		frames = 2000
	}
	for _, size := range sizes {
		for _, classic := range []bool{false, true} {
			for seed := uint64(1); seed <= 3; seed++ {
				name := fmt.Sprintf("%dx%d/classic=%v/seed=%d", size[0], size[1], classic, seed)
				t.Run(name, func(t *testing.T) {
					w, h := size[0], size[1]
					q, a := newTestAquarium(w, h, Options{Classic: classic}, seed)
					now := time.Unix(1_000_000, 0)
					a.Now = func() time.Time { return now }
					q.Populate()

					// Everything the scene must keep: water lines, castle,
					// seaweed, fish and one random creature.
					floor := 4 + 1 + w/15 + max(0, (h-9)*w/350) + 1
					ceiling := floor + 400
					canvas := &recorder{t: t, w: w, h: h}

					for i := range frames {
						a.Animate()
						a.Draw(canvas)
						now = now.Add(100 * time.Millisecond)
						if n := a.EntityCount(); n < floor || n > ceiling {
							t.Fatalf("frame %d: %d entities, want %d-%d", i, n, floor, ceiling)
						}
					}
				})
			}
		}
	}
}

func TestEveryCreatureCanBeAdded(t *testing.T) {
	for _, classic := range []bool{false, true} {
		q, a := newTestAquarium(100, 30, Options{Classic: classic}, 11)
		adders := map[string]func(){
			"ship": q.addShip, "whale": q.addWhale, "monster": q.addMonster,
			"bigFish": q.addBigFish, "bigFish1": q.addBigFish1, "bigFish2": q.addBigFish2,
			"shark": q.addShark, "fish": q.addFish,
			"submarine": q.addSubmarine, "jellyfish": q.addJellyfish,
		}
		for name, add := range adders {
			for range 50 { // both directions, all variants
				a.RemoveAll()
				add()
				if a.EntityCount() == 0 {
					t.Errorf("%s (classic=%v) added nothing", name, classic)
				}
				canvas := &recorder{t: t, w: 100, h: 30}
				for range 150 {
					a.Animate()
					a.Draw(canvas)
				}
			}
		}
	}
}
