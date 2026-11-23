// SPDX-License-Identifier: GPL-2.0-or-later

// Package anim is a small ASCII sprite animation engine. It is a Go port of
// the part of Perl's Term::Animation (Kirk Baucom) that asciiquarium uses:
// entities with frames, colors, depth ordering, collision detection and the
// different ways an entity can die. Drawing goes to a Canvas, so the engine
// knows nothing about terminals.
package anim

import (
	"cmp"
	"slices"
	"time"
)

// Canvas is what an Animation draws on.
type Canvas interface {
	Clear()
	SetCell(x, y int, r rune, c Color)
}

// Animation owns the entities and advances them frame by frame.
type Animation struct {
	// Now is the clock used for DieTime. It can be replaced in tests.
	Now func() time.Time

	width, height int
	entities      []*Entity // in insertion order
	queue         []*Entity // entities waiting to be removed
}

// New creates an empty animation for a screen of the given size.
func New(width, height int) *Animation {
	return &Animation{Now: time.Now, width: width, height: height}
}

// Width and Height are the size of the animation area.
func (a *Animation) Width() int  { return a.width }
func (a *Animation) Height() int { return a.height }

// Resize changes the animation area. Existing entities are left alone; the
// caller usually follows up with RemoveAll and builds the scene again.
func (a *Animation) Resize(width, height int) { a.width, a.height = width, height }

// Add puts an entity in the animation and returns it.
func (a *Animation) Add(e *Entity) *Entity {
	e.anim = a
	a.entities = append(a.entities, e)
	return e
}

// Spawn creates an entity from cfg and adds it.
func (a *Animation) Spawn(cfg Config) *Entity { return a.Add(NewEntity(cfg)) }

// Delete schedules an entity for removal. Its death callback runs at the end
// of the frame. Deleting an entity that is already gone, or that is already
// scheduled, is harmless.
func (a *Animation) Delete(e *Entity) {
	if e.anim == a {
		a.queue = append(a.queue, e)
	}
}

// RemoveAll drops every entity without running any death callback.
func (a *Animation) RemoveAll() {
	for _, e := range a.entities {
		e.anim = nil
	}
	a.entities = nil
	a.queue = nil
}

// EntityCount is the number of entities in the animation.
func (a *Animation) EntityCount() int { return len(a.entities) }

// EntitiesOfType lists the entities with the given type.
func (a *Animation) EntitiesOfType(typ string) []*Entity {
	var out []*Entity
	for _, e := range a.entities {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// Animate runs one animation cycle: callbacks, collision detection and
// handlers, then removal of dead entities. It does not draw.
func (a *Animation) Animate() {
	a.runCallbacks()
	a.handleCollisions()
	a.removeDeleted()
}

// runCallbacks applies the automatic death rules and runs the callback of
// every entity. Entities created along the way are not run until next frame.
func (a *Animation) runCallbacks() {
	now := a.Now().Unix()
	for _, e := range slices.Clone(a.entities) {
		if !e.DieTime.IsZero() && e.DieTime.Unix() <= now {
			a.Delete(e)
			continue
		}
		if e.dieFrame != nil {
			left := *e.dieFrame
			*e.dieFrame--
			if left <= 0 {
				a.Delete(e)
				continue
			}
		}
		if e.DieOffscreen && a.offscreen(e) {
			a.Delete(e)
			continue
		}
		if e.Callback != nil {
			e.Callback(e, a)
		} else {
			e.MoveEntity(a)
		}
	}
}

// offscreen reports whether an entity is entirely outside the screen.
func (a *Animation) offscreen(e *Entity) bool {
	return e.X >= float64(a.width) || e.Y >= float64(a.height) ||
		e.X < float64(-e.width) || e.Y < float64(-e.height)
}

// handleCollisions finds every pair of overlapping physical entities and
// calls the handlers of the entities involved.
func (a *Animation) handleCollisions() {
	var physical []*Entity
	for _, e := range a.entities {
		if e.Physical {
			physical = append(physical, e)
		}
	}
	for k, e := range physical {
		for _, other := range physical[:k] {
			if overlap(other, e) {
				e.collisions = append(e.collisions, other)
				other.collisions = append(other.collisions, e)
			}
		}
	}
	for _, e := range slices.Clone(a.entities) {
		if e.CollisionHandler != nil && len(e.collisions) > 0 {
			e.CollisionHandler(e, a)
		}
		e.collisions = nil
	}
}

// overlap is true if the boxes of two entities intersect on all three axes.
func overlap(p, q *Entity) bool {
	return overlap1(p.X, float64(p.width), q.X, float64(q.width)) &&
		overlap1(p.Y, float64(p.height), q.Y, float64(q.height)) &&
		overlap1(p.Z, float64(p.Depth), q.Z, float64(q.Depth))
}

func overlap1(a, sizeA, b, sizeB float64) bool {
	return (a <= b && b < a+sizeA) || (b <= a && a < b+sizeB)
}

// removeDeleted removes the entities queued by Delete. Death callbacks run
// while the entity is still present and may create entities or delete others.
func (a *Animation) removeDeleted() {
	for len(a.queue) > 0 {
		e := a.queue[0]
		a.queue = a.queue[1:]
		if e.anim != a { // queued twice, already gone
			continue
		}
		if e.DeathCallback != nil {
			e.DeathCallback(e, a)
		}
		e.anim = nil
	}
	a.entities = slices.DeleteFunc(a.entities, func(e *Entity) bool { return e.anim != a })
}

// Draw clears the canvas and draws all entities, farthest (highest Z) first
// so that entities with a lower Z end up on top.
func (a *Animation) Draw(c Canvas) {
	c.Clear()
	sorted := slices.Clone(a.entities)
	slices.SortStableFunc(sorted, func(p, q *Entity) int { return cmp.Compare(q.Z, p.Z) })
	for _, e := range sorted {
		a.draw(c, e)
	}
}

func (a *Animation) draw(c Canvas, e *Entity) {
	f := e.Frame()
	w, h := float64(a.width), float64(a.height)
	for i, row := range e.frames[f] {
		y := e.Y + float64(i)
		for j, ch := range row {
			if ch == e.Transparent {
				continue
			}
			x := e.X + float64(j)
			// Cells are placed by truncating the float position, but the
			// bounds are checked before that, as Term::Animation does.
			if x < 0 || y < 0 || x >= w || y >= h {
				continue
			}
			c.SetCell(int(x), int(y), ch, e.colors[f][i][j])
		}
	}
}
