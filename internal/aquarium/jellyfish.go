// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import "go-asciiquarium/internal/anim"

// addJellyfish adds a jellyfish (pull request #25 of the original). It drifts
// sideways while slowly sinking, at a random depth among the fish.
func (q *Aquarium) addJellyfish() {
	dir := q.irand(2)
	speed := q.rand(1) + .5
	x := 0.0 // unlike the fish, it appears at the edge, not outside the screen
	if dir == 1 {
		speed = -speed
		x = float64(q.a.Width() - 2)
	}
	const maxHeight = 9
	minHeight := q.a.Height() - 10
	q.a.Spawn(anim.Config{
		Type:          "jellyfish",
		Shape:         []string{jellyfishImage[dir]},
		AutoTrans:     true,
		Mask:          []string{q.randColor(jellyfishMask[dir])},
		X:             x,
		Y:             float64(q.irand(minHeight-maxHeight) + maxHeight),
		Z:             float64(q.irand(10) + zFishStart),
		Delta:         anim.Delta{X: speed, Y: .2, Frame: .2},
		DieOffscreen:  true,
		DeathCallback: q.respawnRandomObject,
		DefaultColor:  "MAGENTA",
	})
}
