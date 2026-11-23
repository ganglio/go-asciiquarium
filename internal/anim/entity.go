// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import (
	"math"
	"time"
)

const (
	// defaultDepth is how many Z levels an entity takes for collisions.
	defaultDepth = 1
	// defaultShape is the shape of an entity that has none.
	defaultShape = "*"
)

// Callback is the signature shared by every per-entity hook: the per-step
// callback, the collision handler and the death callback.
type Callback func(e *Entity, a *Animation)

// Delta is how far the default callback (MoveEntity) moves an entity each
// step, and how fast it steps through the animation frames. Fractional
// values are fine: positions are floats and are truncated when drawn.
type Delta struct {
	X, Y, Z float64
	Frame   float64
}

// Config describes a new entity. The zero value of every field is a sensible
// default.
type Config struct {
	Type string // free-form label, used to find entities and in collision handlers

	// Shape is one multi-line string per animation frame.
	Shape []string

	X, Y, Z float64

	// Physical entities take part in collision detection. Depth is how many
	// Z levels the entity occupies for that purpose (defaultDepth if zero).
	Physical         bool
	Depth            int
	CollisionHandler Callback

	// Callback runs once per step. If nil, MoveEntity is used, which does
	// nothing for an entity whose Delta is zero.
	Callback Callback
	Delta    Delta

	// Ways to die. DeathCallback runs when the entity is removed.
	DieOffscreen  bool
	DieTime       time.Time
	DeathCallback Callback
}

// Entity is a sprite: one or more frames of ASCII art with a position and
// some behavior.
type Entity struct {
	Type    string
	X, Y, Z float64
	Depth   int

	Physical         bool
	CollisionHandler Callback
	Callback         Callback
	Delta            Delta

	DieOffscreen  bool
	DieTime       time.Time
	DeathCallback Callback

	frames        [][][]rune // [frame][row][col]
	width, height int
	frame         float64 // fractional; the frame shown is its integer part

	collisions []*Entity
	anim       *Animation // set while the entity belongs to an animation
}

// NewEntity builds an entity from a Config.
func NewEntity(cfg Config) *Entity {
	e := &Entity{
		Type:             cfg.Type,
		X:                cfg.X,
		Y:                cfg.Y,
		Z:                cfg.Z,
		Depth:            cfg.Depth,
		Physical:         cfg.Physical,
		CollisionHandler: cfg.CollisionHandler,
		Callback:         cfg.Callback,
		Delta:            cfg.Delta,
		DieOffscreen:     cfg.DieOffscreen,
		DieTime:          cfg.DieTime,
		DeathCallback:    cfg.DeathCallback,
	}
	if e.Depth == 0 {
		e.Depth = defaultDepth
	}
	shape := cfg.Shape
	if len(shape) == 0 {
		shape = []string{defaultShape}
	}
	e.frames, e.height, e.width = buildFrames(shape)
	return e
}

// Width and Height are the size of the entity in cells, the maximum over all
// its frames.
func (e *Entity) Width() int  { return e.width }
func (e *Entity) Height() int { return e.height }

// Frame is the index of the frame currently shown.
func (e *Entity) Frame() int { return mod(int(e.frame), len(e.frames)) }

// SetFrame shows the given frame (taken modulo the number of frames), for
// callbacks that drive the animation themselves.
func (e *Entity) SetFrame(i int) { e.frame = float64(mod(i, len(e.frames))) }

// Collisions lists the entities this one overlapped with in the current
// step. It is only meaningful inside a collision handler.
func (e *Entity) Collisions() []*Entity { return e.collisions }

// Kill schedules the entity for removal at the end of the current step.
func (e *Entity) Kill() {
	if e.anim != nil {
		e.anim.Delete(e)
	}
}

// MoveEntity is the default callback: it moves the entity by Delta and
// advances its animation frame.
func (e *Entity) MoveEntity(*Animation) {
	if e.Delta.Frame != 0 {
		f := e.frame + e.Delta.Frame
		// Keep the fractional part, wrap the integer part around the number
		// of frames.
		e.frame = (f - math.Trunc(f)) + float64(mod(int(f), len(e.frames)))
	}
	e.X += e.Delta.X
	e.Y += e.Delta.Y
	e.Z += e.Delta.Z
}

// mod is the modulo operator with a non-negative result for positive n.
func mod(a, n int) int {
	return ((a % n) + n) % n
}
