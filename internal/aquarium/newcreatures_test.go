// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"testing"

	"go-asciiquarium/internal/anim"
)

// creature adds one creature of the given type to an empty aquarium and
// returns it. Seeds are tried until it faces the wanted way.
func creature(t *testing.T, w, h int, typ string, goesRight bool, add func(*Aquarium)) (*anim.Animation, *anim.Entity) {
	t.Helper()
	for seed := uint64(0); seed < 200; seed++ {
		q, a := newTestAquarium(w, h, Options{}, seed)
		add(q)
		if es := a.EntitiesOfType(typ); len(es) == 1 && (es[0].Delta.X > 0) == goesRight {
			return a, es[0]
		}
	}
	t.Fatalf("no seed made a %s going right=%v", typ, goesRight)
	return nil, nil
}

func TestSubmarineStopsToRaiseItsPeriscope(t *testing.T) {
	// The center is at width/2-20: exactly on a cell for even widths and
	// half a cell off for odd ones, where the submarine has to stop beside it.
	for _, width := range []int{80, 81, 120, 121} {
		for _, goesRight := range []bool{true, false} {
			t.Run(fmt.Sprintf("width=%d/right=%v", width, goesRight), func(t *testing.T) {
				a, sub := creature(t, width, 30, "submarine", goesRight, (*Aquarium).addSubmarine)

				type tick struct {
					x     float64
					frame int
				}
				var ticks []tick
				for range 400 {
					a.Animate()
					if sub.X >= float64(width) || sub.X < -float64(sub.Width()) {
						break // it is leaving: a dead entity stays where it died
					}
					ticks = append(ticks, tick{sub.X, sub.Frame()})
				}

				// Find the longest stretch where it does not move.
				best, run, start := 0, 1, 0
				for i := 1; i < len(ticks); i++ {
					if ticks[i].x == ticks[i-1].x {
						run++
					} else {
						run = 1
					}
					if run > best {
						best, start = run, i-run+1
					}
				}
				stop := ticks[start : start+best]

				center := float64(width)/2 - 20
				if d := stop[0].x - center; d < -0.5 || d > 0.5 {
					t.Errorf("stopped at x=%v, the center is %v", stop[0].x, center)
				}

				// Frames 0..8 in order, each held for hold+1 ticks. The tick
				// that arrives counts for frame 0, and the periscope is down
				// again right after the tick that reaches frame 8.
				var held [9]int
				last := 0
				for _, tk := range stop {
					if tk.frame < last {
						t.Fatalf("frame went back from %d to %d", last, tk.frame)
					}
					last = tk.frame
					held[tk.frame]++
				}
				if want := [9]int{2, 5, 5, 10, 10, 10, 5, 5, 1}; held != want {
					t.Errorf("ticks per frame = %v, want %v", held, want)
				}

				// Afterwards it carries on in the same direction.
				if start+best >= len(ticks) {
					t.Fatal("simulation too short to see it leave")
				}
				if next := ticks[start+best].x; (next > stop[0].x) != goesRight {
					t.Errorf("did not carry on in its direction: %v -> %v", stop[0].x, next)
				}
			})
		}
	}
}

func TestSubmarineFrames(t *testing.T) {
	for dir := range 2 {
		if len(submarineImage[dir]) != len(submarineHold) {
			t.Errorf("dir %d has %d frames but %d hold times", dir, len(submarineImage[dir]), len(submarineHold))
		}
		// The hull is always on the bottom rows, whatever the periscope does.
		for f, img := range submarineImage[dir] {
			e := anim.NewEntity(anim.Config{
				Shape: []string{img}, AutoTrans: true, Mask: []string{submarineMask[dir]},
			})
			if e.Height() != 10 {
				t.Errorf("dir %d frame %d is %d rows high, want 10", dir, f, e.Height())
			}
		}
	}
}

func TestJellyfish(t *testing.T) {
	for _, goesRight := range []bool{true, false} {
		a, j := creature(t, 100, 30, "jellyfish", goesRight, (*Aquarium).addJellyfish)

		if j.Height() != 6 || j.Width() != 7 {
			t.Errorf("jellyfish is %dx%d, want 7x6", j.Width(), j.Height())
		}
		if speed := max(j.Delta.X, -j.Delta.X); speed < .5 || speed >= 1.5 {
			t.Errorf("speed %v not in 0.5..1.5", j.Delta.X)
		}
		if j.Z < zFishStart || j.Z >= zFishStart+10 {
			t.Errorf("z = %v, want %d..%d", j.Z, zFishStart, zFishStart+9)
		}
		if j.Y < 9 || j.Y >= 20 {
			t.Errorf("y = %v, want 9..19", j.Y)
		}

		x0, y0 := j.X, j.Y
		for range 10 {
			a.Animate()
		}
		// It drifts sideways and sinks 0.2 rows per frame.
		if dy := j.Y - y0; dy < 1.99 || dy > 2.01 {
			t.Errorf("sank %v rows in 10 frames, want 2", dy)
		}
		if dx := j.X - x0; (dx > 0) != goesRight {
			t.Errorf("moved %v, goesRight=%v", dx, goesRight)
		}
	}
}

func TestJellyfishLeavesAndIsReplaced(t *testing.T) {
	a, j := creature(t, 50, 20, "jellyfish", true, (*Aquarium).addJellyfish)
	for range 300 {
		a.Animate()
	}
	// A dead entity is no longer moved, so it is frozen where it left.
	if j.X < 50 && j.Y < 20 {
		t.Errorf("jellyfish still on screen at (%v,%v)", j.X, j.Y)
	}
	if a.EntityCount() == 0 {
		t.Error("nothing took the jellyfish's place")
	}
}

func TestClassicModeHasNeitherSubmarineNorJellyfish(t *testing.T) {
	count := func(opts Options) (subs, jellies int) {
		q, a := newTestAquarium(100, 30, opts, 5)
		for range 600 {
			a.RemoveAll()
			q.randomObject()
			subs += len(a.EntitiesOfType("submarine"))
			jellies += len(a.EntitiesOfType("jellyfish"))
		}
		return subs, jellies
	}
	if s, j := count(Options{Classic: true}); s != 0 || j != 0 {
		t.Errorf("classic mode produced %d submarines and %d jellyfish", s, j)
	}
	// Each of the 7 creatures comes up about one time in 7: 86 out of 600.
	s, j := count(Options{})
	for name, n := range map[string]int{"submarine": s, "jellyfish": j} {
		if n < 50 || n > 130 {
			t.Errorf("%s came up %d times out of 600, expected about 86", name, n)
		}
	}
}

func TestNewCreaturesOnAnyScreenSize(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {20, 8}, {5, 3}} {
		q, a := newTestAquarium(size[0], size[1], Options{}, 1)
		for _, add := range []func(){q.addSubmarine, q.addJellyfish} {
			for range 20 {
				a.RemoveAll()
				add()
				canvas := &recorder{t: t, w: size[0], h: size[1]}
				for range 300 {
					a.Animate()
					a.Draw(canvas)
				}
			}
		}
	}
}
