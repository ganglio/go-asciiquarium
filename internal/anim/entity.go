// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import (
	"fmt"
	"math"
	"time"
)

// Callback is the signature shared by every per-entity hook: the per-frame
// callback, the collision handler and the death callback.
type Callback func(e *Entity, a *Animation)

// Delta is how far the default callback (MoveEntity) moves an entity each
// frame, and how fast it steps through the animation frames. Fractional
// values are fine: positions are floats and are truncated when drawn.
type Delta struct {
	X, Y, Z float64
	Frame   float64
}

// Config describes a new entity. The zero value of every field is a sensible
// default, mirroring Term::Animation::Entity->new.
type Config struct {
	Type string // free-form label, used to find entities and in collision handlers

	// Shape is one multi-line string per animation frame. Mask holds the
	// matching color masks (one letter per cell, see ParseColor); a single
	// mask is reused for every frame. Cells without a mask letter, or with a
	// space, use DefaultColor (white if empty).
	Shape        []string
	Mask         []string
	DefaultColor string

	// AutoTrans makes the whitespace at the start of every line transparent.
	// Transparent is the transparent character, '?' if zero.
	AutoTrans   bool
	Transparent rune

	X, Y, Z float64

	// Physical entities take part in collision detection. Depth is how many
	// Z levels the entity occupies for that purpose (1 if zero).
	Physical         bool
	Depth            int
	CollisionHandler Callback

	// Callback runs once per frame. If nil, MoveEntity is used, which does
	// nothing for an entity whose Delta is zero.
	Callback Callback
	Delta    Delta

	// Ways to die. DeathCallback runs when the entity is removed.
	DieOffscreen  bool
	DieTime       time.Time
	DieFrame      *int // frames to live; nil means forever
	DeathCallback Callback
}

// Entity is a sprite: one or more frames of ASCII art with colors, a position
// and some behavior.
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

	Transparent rune

	frames        [][][]rune // [frame][row][col]
	colors        [][][]Color
	width, height int
	frame         float64 // fractional; the frame shown is its integer part
	dieFrame      *int

	collisions []*Entity
	anim       *Animation // set while the entity belongs to an animation
}

// NewEntity builds an entity. It panics if Config.DefaultColor or a mask
// letter is not a valid color: those are compiled-in constants, so that is a
// programming error, not a runtime condition.
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
		Transparent:      cfg.Transparent,
	}
	if e.Depth == 0 {
		e.Depth = 1
	}
	if e.Transparent == 0 {
		e.Transparent = '?'
	}
	if cfg.DieFrame != nil {
		n := *cfg.DieFrame
		e.dieFrame = &n
	}

	shape := cfg.Shape
	if len(shape) == 0 {
		shape = []string{"*"}
	}
	if cfg.AutoTrans {
		trans := make([]string, len(shape))
		for i, s := range shape {
			trans[i] = autoTrans(s, e.Transparent)
		}
		shape = trans
	}
	e.frames, e.height, e.width = buildFrames(shape)

	def := Color{Hue: White}
	if cfg.DefaultColor != "" {
		c, ok := ParseColor(cfg.DefaultColor)
		if !ok {
			panic(fmt.Sprintf("anim: invalid default color %q", cfg.DefaultColor))
		}
		def = c
	}
	e.buildColors(cfg.Mask, def)
	return e
}

// buildColors resolves the color of every cell of every frame. If there are
// fewer masks than frames the masks are repeated.
func (e *Entity) buildColors(masks []string, def Color) {
	maskFrames, _, _ := buildFrames(masks)
	e.colors = make([][][]Color, len(e.frames))
	for f, grid := range e.frames {
		var mask [][]rune
		if len(maskFrames) > 0 {
			mask = maskFrames[f%len(maskFrames)]
		}
		e.colors[f] = make([][]Color, len(grid))
		for i, row := range grid {
			e.colors[f][i] = make([]Color, len(row))
			for j := range row {
				c := def
				if i < len(mask) && j < len(mask[i]) && mask[i][j] != ' ' {
					var ok bool
					if c, ok = ParseColor(string(mask[i][j])); !ok {
						panic(fmt.Sprintf("anim: invalid color mask character %q", mask[i][j]))
					}
				}
				e.colors[f][i][j] = c
			}
		}
	}
}

// Width and Height are the size of the entity in cells, the maximum over all
// its frames.
func (e *Entity) Width() int  { return e.width }
func (e *Entity) Height() int { return e.height }

// Frame is the index of the frame currently shown.
func (e *Entity) Frame() int {
	if len(e.frames) == 0 {
		return 0
	}
	return mod(int(e.frame), len(e.frames))
}

// SetFrame shows the given frame (taken modulo the number of frames), for
// callbacks that drive the animation themselves.
func (e *Entity) SetFrame(i int) { e.frame = float64(mod(i, len(e.frames))) }

// Collisions lists the entities this one overlapped with in the current
// frame. It is only meaningful inside a collision handler.
func (e *Entity) Collisions() []*Entity { return e.collisions }

// Kill schedules the entity for removal at the end of the current frame.
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
		// Same arithmetic as Term::Animation: keep the fractional part, wrap
		// the integer part around the number of frames.
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
