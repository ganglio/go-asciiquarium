// SPDX-License-Identifier: GPL-2.0-or-later

// Package aquarium is the scene of asciiquarium: water, castle, seaweed, fish
// and the occasional ship, whale, sea monster, big fish or shark.
//
// It is a port of the Perl script by Kirk Baucom (version 1.1), with ASCII art
// by Joan Stark and the extra species by Claudio Matsuoka.
package aquarium

import (
	"math/rand/v2"
	"strings"
	"time"

	"go-asciiquarium/internal/anim"
)

// Version is the version of the original asciiquarium this port follows.
const Version = "1.1"

// Options select the species.
type Options struct {
	// Classic restricts the aquarium to the species of Asciiquarium 1.0: no
	// newer fish and sea monster, no submarine, no jellyfish.
	Classic bool
}

// Z levels. Entities with a lower Z are drawn in front of those with a higher
// one. The water line layers sit at fixed depths so that ships, whales and
// sea monsters appear to float between them.
const (
	zShark     = 2
	zFishStart = 3
	zFishEnd   = 20
	zSeaweed   = 21
	zCastle    = 22

	zWaterGap3 = 3
	zWaterGap2 = 5
	zWaterGap1 = 7
)

// zWaterLine is the Z of each of the four water line segments, top row first.
var zWaterLine = [4]float64{8, 6, 4, 2}

// Aquarium populates an animation and keeps it populated: when something
// dies, a death callback creates its replacement.
type Aquarium struct {
	a   *anim.Animation
	rng *rand.Rand

	newFish    bool
	newMonster bool
	newObjects bool // submarine and jellyfish
}

// New creates an aquarium that draws into a.
func New(a *anim.Animation, opts Options, rng *rand.Rand) *Aquarium {
	return &Aquarium{
		a:          a,
		rng:        rng,
		newFish:    !opts.Classic,
		newMonster: !opts.Classic,
		newObjects: !opts.Classic,
	}
}

// Populate fills the animation with the water, the castle, seaweed, fish and
// one random creature. Call it again after RemoveAll to start over.
func (q *Aquarium) Populate() {
	q.addEnvironment()
	q.addCastle()
	q.addAllSeaweed()
	q.addAllFish()
	q.randomObject()
}

// rand mimics Perl's rand(n): a float in [0, n), where rand(0) means rand(1).
// A negative n gives a value in (n, 0], which matters on tiny terminals.
func (q *Aquarium) rand(n float64) float64 {
	if n == 0 {
		n = 1
	}
	return q.rng.Float64() * n
}

// irand is Perl's int(rand(n)).
func (q *Aquarium) irand(n int) int { return int(q.rand(float64(n))) }

// randColor replaces the digits 1-9 of a color mask by random colors, one per
// digit, so every fish gets its own palette.
func (q *Aquarium) randColor(mask string) string {
	// The original picks index int(rand($#colors)), which never selects the
	// last entry ('M'). Keep that.
	const colors = "cCrRyYbBgGmM"
	for d := '1'; d <= '9'; d++ {
		c := colors[q.irand(len(colors)-1)]
		mask = strings.ReplaceAll(mask, string(d), string(c))
	}
	return mask
}

func (q *Aquarium) addEnvironment() {
	segmentSize := len(waterLineSegment[0])
	repeat := q.a.Width()/segmentSize + 1
	for i, segment := range waterLineSegment {
		q.a.Spawn(anim.Config{
			Type:         "waterline",
			Shape:        []string{strings.Repeat(segment, repeat)},
			X:            0,
			Y:            float64(i + 5),
			Z:            zWaterLine[i],
			DefaultColor: "cyan",
			Depth:        22,
			Physical:     true,
		})
	}
}

func (q *Aquarium) addCastle() {
	q.a.Spawn(anim.Config{
		Shape:        []string{castleImage},
		Mask:         []string{castleMask},
		X:            float64(q.a.Width() - 32),
		Y:            float64(q.a.Height() - 13),
		Z:            zCastle,
		DefaultColor: "BLACK",
	})
}

func (q *Aquarium) addAllSeaweed() {
	for range q.a.Width() / 15 {
		q.addSeaweed()
	}
}

func (q *Aquarium) respawnSeaweed(*anim.Entity, *anim.Animation) { q.addSeaweed() }

func (q *Aquarium) addSeaweed() {
	height := q.irand(4) + 3
	// Two frames that sway: the stalk alternates between "(" and " )" per row.
	var frames [2]strings.Builder
	for i := 1; i <= height; i++ {
		left := i % 2
		frames[left].WriteString("(\n")
		frames[1-left].WriteString(" )\n")
	}
	lifetime := time.Duration(q.irand(4*60)+8*60) * time.Second // 8 to 12 minutes
	q.a.Spawn(anim.Config{
		Shape:         []string{frames[0].String(), frames[1].String()},
		X:             float64(q.irand(q.a.Width()-2) + 1),
		Y:             float64(q.a.Height() - height),
		Z:             zSeaweed,
		Delta:         anim.Delta{Frame: q.rand(.05) + .25},
		DieTime:       q.a.Now().Add(lifetime),
		DeathCallback: q.respawnSeaweed,
		DefaultColor:  "green",
	})
}

// addBubble releases an air bubble from a fish.
func (q *Aquarium) addBubble(fish *anim.Entity) {
	x, y, z := fish.X, fish.Y, fish.Z
	if fish.Delta.X > 0 { // moving right: the mouth is at the right edge
		x += float64(fish.Width())
	}
	y += float64(fish.Height() / 2)
	z-- // a bubble always goes in front of its fish
	q.a.Spawn(anim.Config{
		Type:             "bubble",
		Shape:            []string{".", "o", "O", "O", "O"},
		X:                x,
		Y:                y,
		Z:                z,
		Delta:            anim.Delta{Y: -1, Frame: .1},
		DieOffscreen:     true,
		Physical:         true,
		CollisionHandler: bubbleCollision,
		DefaultColor:     "CYAN",
	})
}

// bubbleCollision pops a bubble when it reaches the water line.
func bubbleCollision(bubble *anim.Entity, _ *anim.Animation) {
	for _, other := range bubble.Collisions() {
		if other.Type == "waterline" {
			bubble.Kill()
			return
		}
	}
}

// addSplat shows the remains of a fish eaten at (x, y, z).
func (q *Aquarium) addSplat(x, y, z float64) {
	q.a.Spawn(anim.Config{
		Shape:        splatArt,
		X:            x - 4,
		Y:            y - 2,
		Z:            z - 2,
		DefaultColor: "RED",
		Delta:        anim.Delta{Frame: .25},
		Transparent:  ' ',
		DieFrame:     new(15),
	})
}

// randomObject adds one of the big, rare creatures. Only one is ever on
// screen: each one adds the next when it dies.
func (q *Aquarium) randomObject() {
	objects := []func(){
		q.addShip,
		q.addWhale,
		q.addMonster,
		q.addBigFish,
		q.addShark,
	}
	if q.newObjects {
		objects = append(objects, q.addSubmarine, q.addJellyfish)
	}
	objects[q.irand(len(objects))]()
}

func (q *Aquarium) respawnRandomObject(*anim.Entity, *anim.Animation) { q.randomObject() }
