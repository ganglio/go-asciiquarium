// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import "go-asciiquarium/internal/anim"

// submarineHold is how many extra ticks each submarine frame stays on screen
// while the periscope goes up, looks around and goes down again.
var submarineHold = [...]int{1, 4, 4, 9, 9, 9, 4, 4, 1}

// addSubmarine adds the yellow submarine (pull request #6 of the original):
// it cruises to the middle of the screen, raises its periscope, looks both
// ways, lowers it again and carries on.
func (q *Aquarium) addSubmarine() {
	dir := q.irand(2)
	x := -40.0
	speed := 1.0
	if dir == 1 {
		speed = -speed
		x = float64(q.a.Width() - 2)
	}
	q.a.Spawn(anim.Config{
		Type:          "submarine",
		Shape:         submarineImage[dir],
		AutoTrans:     true,
		Mask:          []string{submarineMask[dir]},
		X:             x,
		Y:             6,
		Z:             zWaterGap3,
		Delta:         anim.Delta{X: speed},
		Callback:      submarineCallback(),
		DeathCallback: q.respawnRandomObject,
		DieOffscreen:  true,
		DefaultColor:  "YELLOW",
	})
}

// submarineCallback returns the callback of one submarine. The frame being
// shown and how long it has been shown are the state of the callback.
func submarineCallback() anim.Callback {
	frame, delay := 0, 0
	return func(e *anim.Entity, a *anim.Animation) {
		center := float64(a.Width())/2 - 20
		dx := e.Delta.X
		// Stop at the center, or just before it if the step would jump over
		// it (the center is half a cell off on screens of odd width).
		skipsCenter := (e.X < center && e.X+dx > center) || (e.X > center && e.X+dx < center)

		switch {
		case !skipsCenter && e.X != center:
			e.X += dx
		case frame < len(submarineHold)-1:
			if delay < submarineHold[frame] {
				delay++
			} else {
				delay = 0
				frame++
			}
		default: // periscope is down again
			e.X += dx
		}
		e.SetFrame(frame)
	}
}
