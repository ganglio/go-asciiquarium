// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"

	"go-asciiquarium/internal/app"
)

// runLive shows the tank in a terminal in real time: a step of the animation
// every app.StepDuration, drawn each time.
func runLive(tank *app.App, night app.Night, seed uint64, clockOffset time.Duration) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()
	showLive(screen, tank, night, seed, clockOffset)
	return nil
}

// showLive is runLive on a screen that is already there.
func showLive(screen tcell.Screen, tank *app.App, night app.Night, seed uint64, clockOffset time.Duration) {
	env := &terminal{offset: clockOffset, night: night, skipBy: -liveSkip}
	stepsPerSecond := int(time.Second / app.StepDuration)

	draw := func() {
		tank.Draw(tcellCanvas{screen: screen, negative: tank.Asleep()})
		label := "day"
		switch left := tank.PeekLeft(); {
		case left > 0:
			label = fmt.Sprintf("night, peeking at the day: asleep again in %d s, f does nothing until then", int((left+time.Second-1)/time.Second))
		case tank.Asleep():
			label = "night, f peeks at the day"
		}
		now, _ := env.Clock()
		showStatus(screen, fmt.Sprintf("real time, %d steps a second | %s | clock %s, night %s | seed %d | f feed  n night  d day  r new tank  q quit",
			stepsPerSecond, label, now.Format("15:04:05"), nightLabel(night), seed))
	}

	env.events = pollEvents(screen)
	env.resized = screen.Sync // the next frame draws it again
	env.rebuilt = tank.Rebuild
	env.clockChanged = func() {} // the next frame shows the new time
	tank.RunLive(env, night, draw)
}
