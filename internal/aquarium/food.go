// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"math"

	"go-asciiquarium/internal/anim"
)

// The food.
const (
	maxFood = 100 // pellets the tank holds; dropping more does nothing

	minPellets = 5  // pellets dropped at a time
	maxPellets = 10 // ... at most

	// The pellets fall from the middle of the tank, spread over minSpread cells
	// each side of it, or a spreadDivisor-th of the tank's width if that is more.
	minSpread     = 3
	spreadDivisor = 8

	// A pellet sinks between minFall and minFall+extraFall rows per step.
	minFall   = .1
	extraFall = .2
)

// FoodCount is the number of pellets in the tank, sinking or on the bottom.
func (q *Aquarium) FoodCount() int { return len(q.a.EntitiesOfType("food")) }

// DropFood drops some pellets of food into the tank, from the middle of it
// with a random spread. Each one sinks at its own slow speed and then stays
// on the bottom until the fish eats it.
func (q *Aquarium) DropFood() {
	count := q.FoodCount()
	width := q.a.Width()
	midColumn := width / 2
	spread := max(minSpread, width/spreadDivisor)
	for range minPellets + q.irand(maxPellets-minPellets+1) {
		if count >= maxFood {
			return
		}
		count++
		x := midColumn + q.irand(2*spread+1) - spread
		q.addFood(float64(min(max(x, 0), width-1)), firstWaterRow, minFall+q.rand(extraFall))
	}
}

// addFood adds a pellet sinking vy rows per step. It stops at the bottom row.
func (q *Aquarium) addFood(x, y, vy float64) {
	q.a.Spawn(anim.Config{
		Type:     "food",
		Shape:    []string{"*"},
		X:        x,
		Y:        y,
		Z:        zFish,
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

// nearestFood is the pellet closest to the fish, or nil if there is none.
func (q *Aquarium) nearestFood(fish *anim.Entity) *anim.Entity {
	cx := fish.X + middle(fish)
	var best *anim.Entity
	bestDist := math.Inf(1)
	for _, food := range q.a.EntitiesOfType("food") {
		if d := math.Hypot(food.X-cx, (food.Y-fish.Y)*q.aspect); d < bestDist {
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
