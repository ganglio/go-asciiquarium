// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"strings"

	"go-asciiquarium/internal/anim"
)

func (q *Aquarium) addShark() {
	dir := q.irand(2)
	x := -53.0
	y := float64(q.irand(q.a.Height()-(10+9)) + 9)
	teethX := -9.0
	teethY := y + 7
	speed := 2.0
	if dir == 1 {
		speed = -speed
		x = float64(q.a.Width() - 2)
		teethX = x + 9
	}

	// The shark's bite is a separate, invisible-ish entity that moves with it
	// and spans every fish depth, so that it collides with all fish.
	q.a.Spawn(anim.Config{
		Type:     "teeth",
		Shape:    []string{"*"},
		X:        teethX,
		Y:        teethY,
		Z:        zShark + 1,
		Depth:    zFishEnd - zFishStart,
		Delta:    anim.Delta{X: speed},
		Physical: true,
	})

	q.a.Spawn(anim.Config{
		Type:          "shark",
		Mask:          []string{sharkMask[dir]},
		Shape:         []string{sharkImage[dir]},
		AutoTrans:     true,
		X:             x,
		Y:             y,
		Z:             zShark,
		DefaultColor:  "CYAN",
		Delta:         anim.Delta{X: speed},
		DieOffscreen:  true,
		DeathCallback: q.sharkDeath,
	})
}

// sharkDeath removes the teeth together with the shark.
func (q *Aquarium) sharkDeath(_ *anim.Entity, a *anim.Animation) {
	for _, teeth := range a.EntitiesOfType("teeth") {
		a.Delete(teeth)
	}
	q.randomObject()
}

func (q *Aquarium) addShip() {
	dir := q.irand(2)
	x := -24.0
	speed := 1.0
	if dir == 1 {
		speed = -speed
		x = float64(q.a.Width() - 2)
	}
	q.a.Spawn(anim.Config{
		Mask:          []string{shipMask[dir]},
		Shape:         []string{shipImage[dir]},
		AutoTrans:     true,
		X:             x,
		Y:             0,
		Z:             zWaterGap1,
		DefaultColor:  "WHITE",
		Delta:         anim.Delta{X: speed},
		DieOffscreen:  true,
		DeathCallback: q.respawnRandomObject,
	})
}

func (q *Aquarium) addWhale() {
	dir := q.irand(2)
	speed := 1.0
	var x float64
	var spoutAlign int
	if dir == 1 {
		spoutAlign = 1
		speed = -speed
		x = float64(q.a.Width() - 2)
	} else {
		spoutAlign = 11
		x = -18
	}

	// Five frames without a spout, then the spout grows and falls back.
	var shapes, masks []string
	for range 5 {
		shapes = append(shapes, "\n\n\n"+whaleImage[dir])
		masks = append(masks, whaleMask[dir])
	}
	pad := "\n" + strings.Repeat(" ", spoutAlign)
	for _, spout := range waterSpout {
		aligned := strings.Join(anim.Lines(spout), pad)
		shapes = append(shapes, aligned+whaleImage[dir])
		masks = append(masks, whaleMask[dir])
	}

	q.a.Spawn(anim.Config{
		Mask:          masks,
		Shape:         shapes,
		AutoTrans:     true,
		X:             x,
		Y:             0,
		Z:             zWaterGap2,
		DefaultColor:  "WHITE",
		Delta:         anim.Delta{X: speed, Frame: 1},
		DieOffscreen:  true,
		DeathCallback: q.respawnRandomObject,
	})
}

func (q *Aquarium) addMonster() {
	if q.newMonster {
		q.addMonsterEntity(newMonsterImage, newMonsterMask, 54)
	} else {
		q.addMonsterEntity(oldMonsterImage, oldMonsterMask, 64)
	}
}

// addMonsterEntity adds a sea monster. images[dir] are its animation frames
// and masks[dir] the color mask shared by all of them.
func (q *Aquarium) addMonsterEntity(images [][]string, masks []string, offscreen int) {
	dir := q.irand(2)
	x := float64(-offscreen)
	speed := 2.0
	if dir == 1 {
		speed = -speed
		x = float64(q.a.Width() - 2)
	}
	frames := images[dir]
	mask := make([]string, len(frames))
	for i := range mask {
		mask[i] = masks[dir]
	}
	q.a.Spawn(anim.Config{
		Shape:         frames,
		AutoTrans:     true,
		Mask:          mask,
		X:             x,
		Y:             2,
		Z:             zWaterGap2,
		Delta:         anim.Delta{X: speed, Frame: .25},
		DeathCallback: q.respawnRandomObject,
		DieOffscreen:  true,
		DefaultColor:  "GREEN",
	})
}
