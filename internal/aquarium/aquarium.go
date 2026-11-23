// SPDX-License-Identifier: GPL-2.0-or-later

// Package aquarium is the scene: a line of water surface, algae, bubbles and
// one fish that swims around and can be fed.
//
// It descends from the Perl script asciiquarium (Kirk Baucom, version 1.1):
// the algae and the bubbles are its algae and bubbles.
package aquarium

import (
	"math/rand/v2"
	"strings"
	"time"

	"go-asciiquarium/internal/anim"
)

// Z levels. Entities with a lower Z are drawn in front of those with a higher
// one. The fish and the food share a Z so that they collide; bubbles go one
// level in front of the fish; the water surface reaches back as far as the
// algae so that bubbles collide with it.
const (
	zSurface = 2
	zFish    = 5
	zSeaweed = 21

	surfaceDepth = zSeaweed - zSurface

	// bubbleZ is how many levels in front of its fish a bubble goes.
	bubbleZ = -1
)

// The tank. Row 0 is the water surface; the water starts on the row below.
const (
	surfaceRow    = 0
	firstWaterRow = surfaceRow + 1

	// percent is the scale of chances.
	percent = 100

	// terminalCellAspect is how many times taller than wide a character cell is
	// in a terminal.
	terminalCellAspect = 2
)

// The algae.
const (
	algaeSpacing     = 11 // columns of tank for each stalk
	algaeMargin      = 1  // columns kept free of algae at each edge of the tank
	algaeMinHeight   = 3  // rows
	algaeExtraHeight = 4  // how many heights there are: algaeMinHeight and the next ones
	algaeMinLife     = 8 * time.Minute
	algaeExtraLife   = 4 * time.Minute // a stalk lives between algaeMinLife and this much more

	swayFrames     = 2   // the two shapes a stalk sways between
	swayMinSpeed   = .25 // frames per step
	swayExtraSpeed = .05 // the sway speed is up to this much more

	// algaeBubblePercent is the chance, at each step, that a stalk releases a
	// bubble. With the stalks the tank has this makes a bubble or two rise now
	// and then, so that there are always a few in the water.
	algaeBubblePercent = 6
)

// The bubbles: they grow as they rise, and straight up, never sideways.
var bubbleFrames = []string{".", "o", "O", "O", "O"}

const bubbleRise = -1 // rows per step; up is negative

// bubbleGrowth is how many frames a bubble that starts on row y grows at each
// step, if it takes stepsPerRow steps to rise a row, so that it is full size
// when it gets to the top. The row it starts from counts one more than it is
// high: otherwise the last frame would be one past the full size, which is the
// smallest again.
func bubbleGrowth(y float64, stepsPerRow int) float64 {
	return float64(len(bubbleFrames)) / ((y + 1) * float64(stepsPerRow))
}

// newBubble is a bubble that starts at (x, y) and rises a row every
// stepsPerRow steps.
func newBubble(x, y, z float64, stepsPerRow int) anim.Config {
	return anim.Config{
		Type:         "bubble",
		Shape:        bubbleFrames,
		X:            x,
		Y:            y,
		Z:            z,
		Delta:        anim.Delta{Y: float64(bubbleRise) / float64(stepsPerRow), Frame: bubbleGrowth(y, stepsPerRow)},
		DieOffscreen: true,
	}
}

// Options configure the aquarium.
type Options struct {
	// CellAspect is how many times taller than wide a character cell is. The
	// fish needs it to measure distances: it is about 2 in a terminal and
	// 8/6 on the e-ink screen. Zero means that of a terminal.
	CellAspect float64
}

// Aquarium populates an animation and keeps it populated: when algae dies, a
// death callback grows new algae.
type Aquarium struct {
	a      *anim.Animation
	rng    *rand.Rand
	aspect float64
	night  bool // see SetNight
}

// New creates an aquarium that draws into a.
func New(a *anim.Animation, opts Options, rng *rand.Rand) *Aquarium {
	aspect := opts.CellAspect
	if aspect <= 0 {
		aspect = terminalCellAspect
	}
	return &Aquarium{a: a, rng: rng, aspect: aspect}
}

// Populate fills the animation with the water surface, algae and the fish, or
// just the algae at night. Call it again after RemoveAll to start over.
func (q *Aquarium) Populate() {
	if !q.night {
		q.addSurface()
	}
	q.addAllSeaweed()
	if !q.night {
		q.addFish()
	}
}

// rand mimics Perl's rand(n): a float in [0, n), where rand(0) means rand(1).
// A negative n gives a value in (n, 0], which matters on tiny grids.
func (q *Aquarium) rand(n float64) float64 {
	if n == 0 {
		n = 1
	}
	return q.rng.Float64() * n
}

// irand is Perl's int(rand(n)).
func (q *Aquarium) irand(n int) int { return int(q.rand(float64(n))) }

// middle is how far from the left edge of an entity its middle cell is.
func middle(e *anim.Entity) float64 { return float64(e.Width()-1) / 2 }

// addSurface draws the water surface: one line of '~' on the top row. Bubbles
// pop when they reach it.
func (q *Aquarium) addSurface() {
	q.a.Spawn(anim.Config{
		Type:     "waterline",
		Shape:    []string{strings.Repeat("~", q.a.Width())},
		Y:        surfaceRow,
		Z:        zSurface,
		Depth:    surfaceDepth,
		Physical: true,
	})
}

func (q *Aquarium) addAllSeaweed() {
	for range q.a.Width() / algaeSpacing {
		q.addSeaweed()
	}
}

func (q *Aquarium) respawnSeaweed(*anim.Entity, *anim.Animation) { q.addSeaweed() }

func (q *Aquarium) addSeaweed() {
	height := algaeMinHeight + q.irand(algaeExtraHeight)
	// The stalk sways between two shapes: going down it alternates between
	// "(" and " )", and the other shape has them the other way round.
	var shapes [swayFrames]strings.Builder
	for row := 1; row <= height; row++ {
		left := row % swayFrames
		shapes[left].WriteString("(\n")
		shapes[(left+1)%swayFrames].WriteString(" )\n")
	}
	frames := make([]string, swayFrames)
	for i := range shapes {
		frames[i] = shapes[i].String()
	}
	lifetime := algaeMinLife + time.Duration(q.irand(int(algaeExtraLife/time.Second)))*time.Second
	q.a.Spawn(anim.Config{
		Type:          "algae",
		Shape:         frames,
		X:             float64(algaeMargin + q.irand(q.a.Width()-2*algaeMargin)),
		Y:             float64(q.a.Height() - height),
		Z:             zSeaweed,
		Delta:         anim.Delta{Frame: swayMinSpeed + q.rand(swayExtraSpeed)},
		Callback:      q.algaeStep,
		DieTime:       q.a.Now().Add(lifetime),
		DeathCallback: q.respawnSeaweed,
	})
}

// algaeStep sways a stalk and, by day, now and then lets it release a bubble.
// At night the bubbles are let go by NightBubble, once for each picture.
func (q *Aquarium) algaeStep(stalk *anim.Entity, a *anim.Animation) {
	stalk.MoveEntity(a)
	if !q.night && q.irand(percent) < algaeBubblePercent {
		q.releaseBubble(q.aboveStalk(stalk))
	}
}

// aboveStalk is where a bubble starts to rise from a stalk: on the row above
// its top, over one of the columns it takes up.
func (q *Aquarium) aboveStalk(stalk *anim.Entity) (x, y float64) {
	return stalk.X + float64(q.irand(stalk.Width())), stalk.Y - 1
}

// releaseBubble starts a bubble of the day rising from (x, y), unless that is
// above the water. It pops when it reaches the water surface.
func (q *Aquarium) releaseBubble(x, y float64) {
	if y < firstWaterRow {
		return
	}
	bubble := newBubble(x, y, zFish+bubbleZ, 1)
	bubble.Physical = true
	bubble.CollisionHandler = bubbleCollision
	q.a.Spawn(bubble)
}

// addBubble releases an air bubble from a fish that faces right or left.
func (q *Aquarium) addBubble(fish *anim.Entity, facingRight bool) {
	x, y := fish.X, fish.Y
	if facingRight { // the mouth is at the right edge
		x += float64(fish.Width())
	}
	y += float64(fish.Height() / 2)
	bubble := newBubble(x, y, fish.Z+bubbleZ, 1)
	bubble.Physical = true
	bubble.CollisionHandler = bubbleCollision
	q.a.Spawn(bubble)
}

// bubbleCollision pops a bubble when it reaches the water surface.
func bubbleCollision(bubble *anim.Entity, _ *anim.Animation) {
	for _, other := range bubble.Collisions() {
		if other.Type == "waterline" {
			bubble.Kill()
			return
		}
	}
}
