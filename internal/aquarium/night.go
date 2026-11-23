// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import "go-asciiquarium/internal/anim"

// The night: the fish, the water surface and the food are gone and only the
// algae is left, with bubbles that slowly rise from the bottom.
const (
	// NightStepsPerUpdate is how many animation steps pass between two pictures
	// of the night. The algae sways swayMinSpeed frames a step, or a bit more,
	// so this many steps are about one sway: it looks different in each picture.
	NightStepsPerUpdate = 4

	// nightBubblePercent is the chance, for each picture of the night, that a
	// new bubble starts to rise.
	nightBubblePercent = 30
)

// Night reports whether it is night in the tank.
func (q *Aquarium) Night() bool { return q.night }

// SetNight turns the tank into a night tank, or back into a day tank. At night
// the fish, the water surface, the food and the bubbles of the day are gone;
// by day the night's bubbles are, and a fish is somewhere in the water.
func (q *Aquarium) SetNight(night bool) {
	if night == q.night {
		return
	}
	q.night = night
	if night {
		q.removeAll("fish", "waterline", "food", "bubble")
		return
	}
	q.removeAll("bubble")
	q.addSurface()
	q.addFish()
}

// removeAll takes every entity of the given types out of the tank at once.
func (q *Aquarium) removeAll(types ...string) {
	for _, typ := range types {
		for _, e := range q.a.EntitiesOfType(typ) {
			q.a.Remove(e)
		}
	}
}

// NightBubble now and then lets a bubble start to rise from the bottom of the
// tank. Call it once for each picture of the night.
//
// The bubble goes up a row for each picture, and grows a frame at a time on the
// way so that it is full size at the top, where it leaves the tank.
func (q *Aquarium) NightBubble() {
	if q.irand(percent) >= nightBubblePercent {
		return
	}
	rows := q.a.Height()
	q.a.Spawn(anim.Config{
		Type:         "bubble",
		Shape:        bubbleFrames,
		X:            float64(q.irand(q.a.Width())),
		Y:            float64(rows - 1),
		Z:            zFish + bubbleZ,
		Delta:        anim.Delta{Y: float64(bubbleRise) / NightStepsPerUpdate, Frame: float64(len(bubbleFrames)) / float64(rows*NightStepsPerUpdate)},
		DieOffscreen: true,
	})
}
