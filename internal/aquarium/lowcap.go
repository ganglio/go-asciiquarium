// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"math"
	"slices"
	"strings"

	"go-asciiquarium/internal/anim"
)

// The low-capability mode (Options.Low) is not part of the original script.
// It is a bare-bones aquarium for limited terminals: no colors, a line of
// water surface, algae, bubbles and one fish that swims around freely and
// can be fed.

const (
	// lowZ is the depth of the fish and the food. Both must be the same so
	// that they collide; the bubbles (one layer in front) and the water
	// surface (reaching all the way back) are laid out around it.
	lowZ = 5

	wanderSpeed = .2 // plus up to .4 more, in cells per frame
	chaseSpeed  = 1.0

	maxFood = 100 // pellets in the tank; dropping more does nothing
)

// lowFishArt is the fish facing left and facing right. The second one is the
// mirror image of the first.
var lowFishArt = [2]string{"<º(((((()=<", mirror("<º(((((()=<")}

// mirrored swaps the characters that point one way for the ones that point
// the other way. º and = look the same from both sides.
var mirrored = strings.NewReplacer(
	"<", ">", ">", "<",
	"(", ")", ")", "(",
	"[", "]", "]", "[",
	"{", "}", "}", "{",
	"/", `\`, `\`, "/",
)

// mirror flips a one-line sprite horizontally: it reverses the characters and
// turns the directional ones around.
func mirror(s string) string {
	r := []rune(s)
	slices.Reverse(r)
	return mirrored.Replace(string(r))
}

func (q *Aquarium) populateLow() {
	q.addSurface()
	q.addAllSeaweed()
	q.addLowFish()
}

// addSurface draws the water surface: one line of '~' on the top row.
// Bubbles pop when they reach it.
func (q *Aquarium) addSurface() {
	q.a.Spawn(anim.Config{
		Type:     "waterline",
		Shape:    []string{strings.Repeat("~", q.a.Width())},
		Z:        2,
		Depth:    22,
		Physical: true,
	})
}

// lowFish is the state of the one fish of the low-capability mode.
type lowFish struct {
	q     *Aquarium
	right bool // which way it faces

	// Where it is heading when nothing else interests it, and how fast.
	hasTarget bool
	tx, ty    float64
	speed     float64
}

func (q *Aquarium) addLowFish() {
	f := &lowFish{q: q, right: q.irand(2) == 1}
	fish := anim.NewEntity(anim.Config{
		Type:             "fish",
		Shape:            lowFishArt[:], // frame 0 faces left, frame 1 right
		Z:                lowZ,
		Physical:         true,
		Callback:         f.step,
		CollisionHandler: eatFood,
	})
	maxX, maxY := f.area(fish)
	fish.X = float64(q.irand(maxX + 1))
	fish.Y = float64(1 + q.irand(maxY))
	f.show(fish)
	q.a.Add(fish)
}

// area is the size of the water the fish's top-left corner can be in: its x
// goes from 0 to maxX and its y from 1 (under the surface) to maxY (the
// bottom row).
func (f *lowFish) area(fish *anim.Entity) (maxX, maxY int) {
	return max(0, f.q.a.Width()-fish.Width()), max(1, f.q.a.Height()-1)
}

// show picks the sprite for the way the fish faces. Turning around is just a
// matter of showing the other one.
func (f *lowFish) show(fish *anim.Entity) {
	if f.right {
		fish.SetFrame(1)
	} else {
		fish.SetFrame(0)
	}
}

// step runs once per frame: choose where to go, turn if that is the other
// way, swim a bit, and now and then blow a bubble.
func (f *lowFish) step(fish *anim.Entity, a *anim.Animation) {
	q := f.q
	maxX, maxY := f.area(fish)

	var tx, ty, speed float64
	if food := nearestFood(fish, a); food != nil {
		// Aim the middle of the fish at the pellet; it is eaten as soon as
		// any part of the fish touches it.
		tx = min(max(food.X-float64(fish.Width()-1)/2, 0), float64(maxX))
		ty = min(max(food.Y, 1), float64(maxY))
		speed = chaseSpeed
		f.hasTarget = false // pick a new place to go once the food is gone
	} else {
		// Wander: go to a random spot, sometimes changing its mind on the way.
		if !f.hasTarget || q.irand(100) < 2 {
			f.tx = float64(q.irand(maxX + 1))
			f.ty = float64(1 + q.irand(maxY))
			f.speed = wanderSpeed + q.rand(.4)
			f.hasTarget = true
		}
		tx, ty, speed = f.tx, f.ty, f.speed
	}

	// Turn around when the target is clearly on the other side. The margin
	// keeps it from flipping back and forth when it is right above it.
	if dx := tx - fish.X; dx > 1 {
		f.right = true
	} else if dx < -1 {
		f.right = false
	}
	f.show(fish)

	// Cells are about twice as tall as wide, so a step up or down counts
	// double when measuring distance, and takes twice the time.
	dx, dy := tx-fish.X, (ty-fish.Y)*2
	if dist := math.Hypot(dx, dy); dist <= speed {
		fish.X, fish.Y = tx, ty
		f.hasTarget = false
	} else {
		fish.X += dx / dist * speed
		fish.Y += dy / dist * speed / 2
	}

	if q.irand(100) > 97 {
		q.addBubble(fish, f.right)
	}
}

// nearestFood is the pellet closest to the fish, or nil if there is none.
func nearestFood(fish *anim.Entity, a *anim.Animation) *anim.Entity {
	cx := fish.X + float64(fish.Width()-1)/2
	var best *anim.Entity
	bestDist := math.Inf(1)
	for _, food := range a.EntitiesOfType("food") {
		if d := math.Hypot(food.X-cx, (food.Y-fish.Y)*2); d < bestDist {
			best, bestDist = food, d
		}
	}
	return best
}

// eatFood is the fish's collision handler: it eats the pellets it touches.
func eatFood(fish *anim.Entity, _ *anim.Animation) {
	for _, other := range fish.Collisions() {
		if other.Type == "food" {
			other.Kill()
		}
	}
}

// DropFood drops 5 to 10 pellets of food into the tank, from the middle of
// it with a random spread. Each one sinks at its own slow speed and then
// stays on the bottom until the fish eats it. It does nothing outside the
// low-capability mode, and no more than maxFood pellets are kept.
func (q *Aquarium) DropFood() {
	if !q.low {
		return
	}
	count := len(q.a.EntitiesOfType("food"))
	width := q.a.Width()
	spread := max(3, width/8)
	for range 5 + q.irand(6) {
		if count >= maxFood {
			return
		}
		count++
		x := width/2 + q.irand(2*spread+1) - spread
		q.addFood(float64(min(max(x, 0), width-1)), 1, .1+q.rand(.2))
	}
}

// addFood adds a pellet sinking vy rows per frame. It stops at the bottom row.
func (q *Aquarium) addFood(x, y, vy float64) {
	q.a.Spawn(anim.Config{
		Type:     "food",
		Shape:    []string{"*"},
		X:        x,
		Y:        y,
		Z:        lowZ,
		Physical: true,
		Delta:    anim.Delta{Y: vy},
		Callback: settleAtBottom,
	})
}

// settleAtBottom moves a pellet and stops it once it reaches the bottom.
func settleAtBottom(food *anim.Entity, a *anim.Animation) {
	food.MoveEntity(a)
	if bottom := float64(a.Height() - 1); food.Y >= bottom {
		food.Y = bottom
		food.Delta.Y = 0
	}
}
