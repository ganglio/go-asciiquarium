// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"math"

	"go-asciiquarium/internal/anim"
	"go-asciiquarium/internal/flip"
)

// The fish has one sprite for each way it can face. These are their frame
// numbers.
const (
	frameLeft = iota
	frameRight
)

// fishFacingLeft is the fish as it looks when it swims to the left. This is
// the one place that says what the fish is: the other way round is its mirror
// image, and everything about its size comes from the sprites.
const fishFacingLeft = "<o)))><"

// fishArt is the sprite for each way the fish faces.
var fishArt = [...]string{
	frameLeft:  fishFacingLeft,
	frameRight: flip.String(fishFacingLeft),
}

// How the fish swims, in cells per step unless said otherwise.
const (
	wanderMinSpeed   = .2 // a fish going nowhere in particular
	wanderExtraSpeed = .4 // it swims up to this much faster
	chaseSpeed       = 1.0

	// turnMargin is how many cells beyond it a target must be for the fish to
	// turn towards it. The margin keeps the fish from flipping back and forth
	// when it is right above or below its target.
	turnMargin = 1.0

	// Chances, in percent, per step.
	changeMindPercent = 2 // a wandering fish picks another place to go to
	bubblePercent     = 2 // the fish blows a bubble
)

// fish is the state of the one fish.
type fish struct {
	q     *Aquarium
	right bool // which way it faces

	// Where it is heading when nothing else interests it, and how fast.
	hasTarget bool
	tx, ty    float64
	speed     float64
}

func (q *Aquarium) addFish() {
	f := &fish{q: q, right: q.irand(len(fishArt)) == frameRight}
	e := anim.NewEntity(anim.Config{
		Type:             "fish",
		Shape:            fishArt[:],
		Z:                zFish,
		Physical:         true,
		Callback:         f.step,
		CollisionHandler: eatFood,
	})
	e.X, e.Y = f.somewhere(e)
	f.show(e)
	q.a.Add(e)
}

// area is the size of the water the fish's top-left corner can be in: its x
// goes from 0 to maxX and its y from the first row of water to maxY, the
// bottom row.
func (f *fish) area(e *anim.Entity) (maxX, maxY int) {
	lastRow := f.q.a.Height() - 1
	return max(0, f.q.a.Width()-e.Width()), max(firstWaterRow, lastRow)
}

// somewhere is a random place in the water for the fish's top-left corner.
func (f *fish) somewhere(e *anim.Entity) (x, y float64) {
	maxX, maxY := f.area(e)
	rows := maxY - firstWaterRow + 1
	return float64(f.q.irand(maxX + 1)), float64(firstWaterRow + f.q.irand(rows))
}

// show picks the sprite for the way the fish faces. Turning around is just a
// matter of showing the other one.
func (f *fish) show(e *anim.Entity) {
	if f.right {
		e.SetFrame(frameRight)
	} else {
		e.SetFrame(frameLeft)
	}
}

// step runs once per animation step: choose where to go, turn if that is the
// other way, swim a bit, and now and then blow a bubble.
func (f *fish) step(e *anim.Entity, a *anim.Animation) {
	q := f.q

	var tx, ty, speed float64
	if food := q.nearestFood(e); food != nil {
		// Aim the middle of the fish at the pellet; it is eaten as soon as
		// any part of the fish touches it.
		maxX, maxY := f.area(e)
		tx = min(max(food.X-middle(e), 0), float64(maxX))
		ty = min(max(food.Y, firstWaterRow), float64(maxY))
		speed = chaseSpeed
		f.hasTarget = false // pick a new place to go once the food is gone
	} else {
		// Wander: go to a random spot, sometimes changing its mind on the way.
		if !f.hasTarget || q.irand(percent) < changeMindPercent {
			f.tx, f.ty = f.somewhere(e)
			f.speed = wanderMinSpeed + q.rand(wanderExtraSpeed)
			f.hasTarget = true
		}
		tx, ty, speed = f.tx, f.ty, f.speed
	}

	// Turn around when the target is clearly on the other side.
	if dx := tx - e.X; dx > turnMargin {
		f.right = true
	} else if dx < -turnMargin {
		f.right = false
	}
	f.show(e)

	// Cells are taller than wide, so a step up or down counts more when
	// measuring distance, and takes proportionally longer.
	dx, dy := tx-e.X, (ty-e.Y)*q.aspect
	if dist := math.Hypot(dx, dy); dist <= speed {
		e.X, e.Y = tx, ty
		f.hasTarget = false
	} else {
		e.X += dx / dist * speed
		e.Y += dy / dist * speed / q.aspect
	}

	if q.irand(percent) >= percent-bubblePercent {
		q.addBubble(e, f.right)
	}
}
