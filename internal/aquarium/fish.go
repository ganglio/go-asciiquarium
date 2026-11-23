// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"strings"

	"go-asciiquarium/internal/anim"
)

func (q *Aquarium) addAllFish() {
	// One fish per 350 cells of water (the 9 rows above it don't count).
	size := (q.a.Height() - 9) * q.a.Width()
	for range size / 350 {
		q.addFish()
	}
}

func (q *Aquarium) respawnFish(*anim.Entity, *anim.Animation) { q.addFish() }

func (q *Aquarium) addFish() {
	if q.newFish && q.irand(12) > 8 {
		q.addFishEntity(newFishArt)
	} else {
		q.addFishEntity(oldFishArt)
	}
}

// addFishEntity adds a random fish from art, a list of (image, color mask)
// pairs. Even pairs face right and swim right, odd pairs face left.
//
// The mask digits mean: 1 body, 2 dorsal fin, 3 flippers, 4 eye, 5 mouth,
// 6 tail fin, 7 gills.
func (q *Aquarium) addFishEntity(art []string) {
	fishNum := int(q.rng.Float64() * float64(len(art)-1) / 2)
	index := fishNum * 2
	speed := q.rand(2) + .25
	z := float64(q.irand(zFishEnd-zFishStart) + zFishStart)
	mask := strings.ReplaceAll(art[index+1], "4", "W")
	mask = q.randColor(mask)
	goingLeft := fishNum%2 == 1
	if goingLeft {
		speed = -speed
	}

	fish := anim.NewEntity(anim.Config{
		Type:             "fish",
		Shape:            []string{art[index]},
		AutoTrans:        true,
		Mask:             []string{mask},
		Z:                z,
		Callback:         q.fishCallback,
		Delta:            anim.Delta{X: speed},
		DieOffscreen:     true,
		DeathCallback:    q.respawnFish,
		Physical:         true,
		CollisionHandler: q.fishCollision,
	})

	// Start just outside the screen on the side the fish comes from, at a
	// random depth below the water line.
	const maxHeight = 9
	minHeight := q.a.Height() - fish.Height()
	fish.Y = float64(q.irand(minHeight-maxHeight) + maxHeight)
	if goingLeft {
		fish.X = float64(q.a.Width() - 2)
	} else {
		fish.X = float64(1 - fish.Width())
	}
	q.a.Add(fish)
}

func (q *Aquarium) fishCallback(fish *anim.Entity, a *anim.Animation) {
	if q.irand(100) > 97 {
		q.addBubble(fish)
	}
	fish.MoveEntity(a)
}

// fishCollision lets a shark eat small fish: a fish touching the shark's teeth
// is replaced by a splat. Big fish are safe.
func (q *Aquarium) fishCollision(fish *anim.Entity, _ *anim.Animation) {
	for _, other := range fish.Collisions() {
		if other.Type == "teeth" && fish.Height() <= 5 {
			q.addSplat(other.X, other.Y, other.Z)
			fish.Kill()
			return
		}
	}
}

func (q *Aquarium) addBigFish() {
	if q.newFish && q.irand(3) > 1 {
		q.addBigFish2()
	} else {
		q.addBigFish1()
	}
}

func (q *Aquarium) addBigFish1() {
	q.addBigFishEntity(bigFish1Image, bigFish1Mask, 34, 3, 15)
}

func (q *Aquarium) addBigFish2() {
	q.addBigFishEntity(bigFish2Image, bigFish2Mask, 33, 2.5, 14)
}

// addBigFishEntity adds a big fish swimming at the given speed. images and
// masks hold the right-facing and left-facing versions; offscreen is how far
// left of the screen a right-swimming fish starts, and bottom how many rows
// must stay free below its top row.
func (q *Aquarium) addBigFishEntity(images, masks []string, offscreen int, speed float64, bottom int) {
	dir := q.irand(2)
	x := float64(-offscreen)
	if dir == 1 {
		x = float64(q.a.Width() - 1)
		speed = -speed
	}
	const maxHeight = 9
	minHeight := q.a.Height() - bottom
	q.a.Spawn(anim.Config{
		Shape:         []string{images[dir]},
		AutoTrans:     true,
		Mask:          []string{q.randColor(masks[dir])},
		X:             x,
		Y:             float64(q.irand(minHeight-maxHeight) + maxHeight),
		Z:             zShark,
		Delta:         anim.Delta{X: speed},
		DeathCallback: q.respawnRandomObject,
		DieOffscreen:  true,
		DefaultColor:  "YELLOW",
	})
}
