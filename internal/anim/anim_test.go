// SPDX-License-Identifier: GPL-2.0-or-later

package anim

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// grid is a Canvas that keeps what was drawn.
type grid struct {
	w, h  int
	cells map[[2]int]rune
}

func (g *grid) Clear()                    { g.cells = map[[2]int]rune{} }
func (g *grid) SetCell(x, y int, ch rune) { g.cells[[2]int{x, y}] = ch }

// String renders the grid, '.' for empty cells.
func (g *grid) String() string {
	var b strings.Builder
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if r, ok := g.cells[[2]int{x, y}]; ok {
				b.WriteRune(r)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func draw(a *Animation) *grid {
	g := &grid{w: a.Width(), h: a.Height()}
	a.Draw(g)
	return g
}

func TestLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"\n", nil},
		{"a\nb", []string{"a", "b"}},
		{"a\nb\n\n\n", []string{"a", "b"}},
		{"\n\na\n", []string{"", "", "a"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
	}
	for _, tt := range tests {
		if got := lines(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("lines(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShapeSize(t *testing.T) {
	// Trailing blank lines don't count and the size is the maximum over all
	// frames.
	e := NewEntity(Config{Shape: []string{"ab\nc\n\n", "xyz\n"}})
	if e.Width() != 3 || e.Height() != 2 {
		t.Errorf("size = %dx%d, want 3x2", e.Width(), e.Height())
	}
}

func TestMultiByteCharactersAreOneCell(t *testing.T) {
	e := NewEntity(Config{Shape: []string{"<º(("}})
	if e.Width() != 4 {
		t.Errorf("width = %d, want 4", e.Width())
	}
}

func TestDefaultShapeIsAsterisk(t *testing.T) {
	a := New(3, 1)
	a.Spawn(Config{})
	if got := draw(a).String(); got != "*..\n" {
		t.Errorf("got %q", got)
	}
}

func TestSpacesAreOpaque(t *testing.T) {
	a := New(5, 1)
	a.Spawn(Config{Shape: []string{"#####"}, Z: 5})
	a.Spawn(Config{Shape: []string{" a "}, Z: 1})
	if got := draw(a).String(); got != " a ##\n" {
		t.Errorf("got %q", got)
	}
}

func TestLowerZIsDrawnOnTop(t *testing.T) {
	a := New(3, 1)
	a.Spawn(Config{Shape: []string{"A"}, Z: 1})
	a.Spawn(Config{Shape: []string{"B"}, Z: 2})
	a.Spawn(Config{Shape: []string{"C"}, Z: 0})
	if got := draw(a).String(); got != "C..\n" {
		t.Errorf("got %q", got)
	}
}

func TestEqualZKeepsCreationOrder(t *testing.T) {
	a := New(3, 1)
	a.Spawn(Config{Shape: []string{"A"}, Z: 1})
	a.Spawn(Config{Shape: []string{"B"}, Z: 1})
	if got := draw(a).String(); got != "B..\n" {
		t.Errorf("got %q: the later entity should be on top", got)
	}
}

func TestDrawClipsAndTruncates(t *testing.T) {
	tests := []struct {
		name string
		x, y float64
		want string
	}{
		{"inside", 1, 0, ".abc.\n.....\n"},
		{"left edge", -1, 0, "bc...\n.....\n"},
		// The bounds check uses the float position, the cell its integer part.
		{"fraction left", -0.5, 0, "bc...\n.....\n"},
		{"fraction right", 1.9, 0, ".abc.\n.....\n"},
		{"right edge", 3, 0, "...ab\n.....\n"},
		{"off right", 5, 0, ".....\n.....\n"},
		{"above", 0, -1, ".....\n.....\n"},
		{"below", 0, 2, ".....\n.....\n"},
		{"last row", 0, 1, ".....\nabc..\n"},
	}
	for _, tt := range tests {
		a := New(5, 2)
		a.Spawn(Config{Shape: []string{"abc"}, X: tt.x, Y: tt.y})
		if got := draw(a).String(); got != tt.want {
			t.Errorf("%s: got\n%swant\n%s", tt.name, got, tt.want)
		}
	}
}

func TestMoveEntity(t *testing.T) {
	a := New(80, 24)
	e := a.Spawn(Config{Delta: Delta{X: 2, Y: -1, Z: 0.5}, X: 10, Y: 10, Z: 3})
	a.Animate()
	a.Animate()
	if e.X != 14 || e.Y != 8 || e.Z != 4 {
		t.Errorf("position = %v,%v,%v", e.X, e.Y, e.Z)
	}
}

func TestFrameStepping(t *testing.T) {
	a := New(10, 1)
	e := a.Spawn(Config{Shape: []string{"a", "b", "c"}, Delta: Delta{Frame: 0.25}})
	var got []int
	for range 14 {
		got = append(got, e.Frame())
		a.Animate()
	}
	// Each frame lasts four steps and the sequence wraps around.
	want := []int{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frames = %v, want %v", got, want)
	}
}

func TestWholeFrameSteps(t *testing.T) {
	a := New(10, 1)
	e := a.Spawn(Config{Shape: []string{"a", "b"}, Delta: Delta{Frame: 1}})
	var got []int
	for range 5 {
		got = append(got, e.Frame())
		a.Animate()
	}
	if want := []int{0, 1, 0, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("frames = %v, want %v", got, want)
	}
}

func TestSetFrame(t *testing.T) {
	a := New(10, 1)
	e := a.Spawn(Config{Shape: []string{"a", "b", "c"}})
	for in, want := range map[int]int{0: 0, 2: 2, 3: 0, 7: 1, -1: 2} {
		e.SetFrame(in)
		if e.Frame() != want {
			t.Errorf("SetFrame(%d): frame %d, want %d", in, e.Frame(), want)
		}
	}
	e.SetFrame(1)
	if got := draw(a).String(); got != "b.........\n" {
		t.Errorf("drew %q", got)
	}
}

func TestCustomCallbackReplacesDefaultMovement(t *testing.T) {
	a := New(10, 1)
	calls := 0
	e := a.Spawn(Config{Delta: Delta{X: 1}, Callback: func(*Entity, *Animation) { calls++ }})
	a.Animate()
	if calls != 1 || e.X != 0 {
		t.Errorf("calls=%d x=%v", calls, e.X)
	}
	// A custom callback can still use the default movement.
	e.Callback = func(e *Entity, a *Animation) { e.MoveEntity(a) }
	a.Animate()
	if e.X != 1 {
		t.Errorf("x = %v", e.X)
	}
}

func TestDieTime(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	a := New(10, 1)
	a.Now = func() time.Time { return now }
	a.Spawn(Config{DieTime: now.Add(10 * time.Second)})
	a.Animate()
	if a.EntityCount() != 1 {
		t.Fatal("died too early")
	}
	now = now.Add(9 * time.Second)
	a.Animate()
	if a.EntityCount() != 1 {
		t.Fatal("died too early")
	}
	now = now.Add(time.Second)
	a.Animate()
	if a.EntityCount() != 0 {
		t.Fatal("should be dead")
	}
}

func TestDieOffscreen(t *testing.T) {
	tests := []struct {
		name string
		x, y float64
		dead bool
	}{
		{"on screen", 0, 0, false},
		{"partly left", -2, 0, false},
		{"just on the left edge", -3, 0, false}, // width 3: X >= -width is still alive
		{"gone left", -3.5, 0, true},
		{"gone right", 10, 0, true},
		{"gone below", 0, 5, true},
		{"gone above", 0, -2.5, true}, // height 2
		{"partly right", 9, 0, false},
	}
	for _, tt := range tests {
		a := New(10, 5)
		a.Spawn(Config{Shape: []string{"abc\ndef"}, X: tt.x, Y: tt.y, DieOffscreen: true})
		a.Animate()
		if dead := a.EntityCount() == 0; dead != tt.dead {
			t.Errorf("%s: dead = %v, want %v", tt.name, dead, tt.dead)
		}
	}
}

func TestDeathCallbackRunsOnceAndCanSpawn(t *testing.T) {
	a := New(10, 1)
	deaths := 0
	a.Spawn(Config{DieOffscreen: true, X: 50, DeathCallback: func(e *Entity, a *Animation) {
		deaths++
		a.Spawn(Config{Type: "replacement"})
	}})
	a.Animate()
	if deaths != 1 || len(a.EntitiesOfType("replacement")) != 1 || a.EntityCount() != 1 {
		t.Errorf("deaths=%d replacements=%d total=%d", deaths, len(a.EntitiesOfType("replacement")), a.EntityCount())
	}
}

func TestDeletingTwiceKillsOnce(t *testing.T) {
	a := New(10, 1)
	deaths := 0
	e := a.Spawn(Config{DeathCallback: func(*Entity, *Animation) { deaths++ }})
	a.Delete(e)
	a.Delete(e)
	e.Kill()
	a.Animate()
	if deaths != 1 || a.EntityCount() != 0 {
		t.Errorf("deaths=%d count=%d", deaths, a.EntityCount())
	}
	// Deleting a dead entity is harmless.
	e.Kill()
	a.Animate()
}

func TestDeathCallbackCanDeleteOthers(t *testing.T) {
	a := New(10, 1)
	a.Spawn(Config{Type: "follower"})
	a.Spawn(Config{Type: "follower"})
	a.Spawn(Config{DieOffscreen: true, X: 50, DeathCallback: func(_ *Entity, a *Animation) {
		for _, f := range a.EntitiesOfType("follower") {
			a.Delete(f)
		}
	}})
	a.Animate()
	if a.EntityCount() != 0 {
		t.Errorf("%d entities left", a.EntityCount())
	}
}

func TestNewEntitiesFirstRunNextStep(t *testing.T) {
	a := New(10, 1)
	var child *Entity
	a.Spawn(Config{Callback: func(_ *Entity, a *Animation) {
		if child == nil {
			child = a.Spawn(Config{Delta: Delta{X: 1}})
		}
	}})
	a.Animate()
	if child.X != 0 {
		t.Errorf("child moved in the step it was created: x=%v", child.X)
	}
	a.Animate()
	if child.X != 1 {
		t.Errorf("child x = %v", child.X)
	}
}

func TestRemoveAllSkipsDeathCallbacks(t *testing.T) {
	a := New(10, 1)
	called := false
	e := a.Spawn(Config{DeathCallback: func(*Entity, *Animation) { called = true }})
	a.RemoveAll()
	e.Kill()
	a.Animate()
	if called || a.EntityCount() != 0 {
		t.Errorf("called=%v count=%d", called, a.EntityCount())
	}
}

func TestCollisions(t *testing.T) {
	collide := func(p, q Config) (hits int) {
		a := New(80, 24)
		p.Physical, q.Physical = true, true
		p.CollisionHandler = func(e *Entity, _ *Animation) { hits += len(e.Collisions()) }
		a.Spawn(p)
		a.Spawn(q)
		a.Animate()
		return hits
	}
	box := func(x, y, z float64, depth int) Config {
		return Config{Shape: []string{"abc\nabc"}, X: x, Y: y, Z: z, Depth: depth}
	}

	tests := []struct {
		name string
		q    Config
		want int
	}{
		{"same place", box(0, 0, 0, 1), 1},
		{"overlap in x", box(2, 0, 0, 1), 1},
		{"touching in x", box(3, 0, 0, 1), 0},
		{"overlap in y", box(0, 1, 0, 1), 1},
		{"touching in y", box(0, 2, 0, 1), 0},
		{"different z", box(0, 0, 1, 1), 0},
		{"deep entity reaches z", box(0, 0, -2, 3), 1},
		{"deep entity stops short", box(0, 0, -3, 3), 0},
		{"fractional overlap", box(2.5, 1.5, 0, 1), 1},
	}
	for _, tt := range tests {
		if got := collide(box(0, 0, 0, 1), tt.q); got != tt.want {
			t.Errorf("%s: %d collisions, want %d", tt.name, got, tt.want)
		}
	}
}

func TestNonPhysicalEntitiesDoNotCollide(t *testing.T) {
	a := New(10, 1)
	hits := 0
	a.Spawn(Config{Physical: true, CollisionHandler: func(*Entity, *Animation) { hits++ }})
	a.Spawn(Config{})
	a.Animate()
	if hits != 0 {
		t.Errorf("hits = %d", hits)
	}
}

func TestCollisionsAreResetEveryStep(t *testing.T) {
	a := New(10, 1)
	var seen []int
	handler := func(e *Entity, _ *Animation) { seen = append(seen, len(e.Collisions())) }
	a.Spawn(Config{Physical: true, CollisionHandler: handler})
	mover := a.Spawn(Config{Physical: true, Delta: Delta{X: 1}})
	a.Animate() // mover is at x=1: no overlap with the 1-cell entity at x=0
	if len(seen) != 0 {
		t.Fatalf("unexpected collision: %v", seen)
	}
	mover.X = -1
	a.Animate() // moves to x=0: overlap
	mover.X = 5
	a.Animate()
	a.Animate()
	if !reflect.DeepEqual(seen, []int{1}) {
		t.Errorf("collision counts = %v, want [1]", seen)
	}
}

func TestCollisionHandlerCanKillAndSpawn(t *testing.T) {
	a := New(10, 1)
	a.Spawn(Config{Type: "a", Physical: true, CollisionHandler: func(e *Entity, a *Animation) {
		a.Spawn(Config{Type: "spawned"})
		e.Kill()
	}})
	a.Spawn(Config{Type: "b", Physical: true})
	a.Animate()
	if len(a.EntitiesOfType("a")) != 0 || len(a.EntitiesOfType("b")) != 1 || len(a.EntitiesOfType("spawned")) != 1 {
		t.Errorf("a=%d b=%d spawned=%d", len(a.EntitiesOfType("a")), len(a.EntitiesOfType("b")), len(a.EntitiesOfType("spawned")))
	}
}

func TestRemoveIsImmediateAndSkipsTheDeathCallback(t *testing.T) {
	a := New(10, 1)
	called := false
	e := a.Spawn(Config{Shape: []string{"x"}, DeathCallback: func(*Entity, *Animation) { called = true }})
	a.Spawn(Config{Shape: []string{"y"}, X: 1})
	a.Remove(e)
	if got := draw(a).String(); got != ".y........\n" {
		t.Errorf("the removed entity was still drawn: %q", got)
	}
	a.Remove(e) // harmless
	a.Animate()
	if called || a.EntityCount() != 1 {
		t.Errorf("called=%v count=%d", called, a.EntityCount())
	}
	e.Kill() // harmless too
	a.Animate()
}

func TestRemoveAfterDeleteDoesNotRunTheDeathCallback(t *testing.T) {
	a := New(10, 1)
	called := false
	e := a.Spawn(Config{DeathCallback: func(*Entity, *Animation) { called = true }})
	a.Delete(e)
	a.Remove(e)
	a.Animate()
	if called || a.EntityCount() != 0 {
		t.Errorf("called=%v count=%d", called, a.EntityCount())
	}
}
