// SPDX-License-Identifier: GPL-2.0-or-later

package aquarium

import (
	"fmt"
	"strings"
	"testing"

	"go-asciiquarium/internal/anim"
)

// rows returns the rows of a multi-line sprite string as the engine sees them.
func rows(s string) []string { return anim.Lines(strings.TrimPrefix(s, "\n")) }

// checkSprite builds an entity from an image and its mask (this panics on an
// invalid mask letter) and checks that the mask does not stick out of the image.
func checkSprite(t *testing.T, name, image, mask string) {
	t.Helper()
	q, _ := newTestAquarium(80, 24, Options{}, 1)
	// Masks of randomly colored sprites use the digits 1-9.
	coloured := q.randColor(strings.ReplaceAll(mask, "4", "W"))
	anim.NewEntity(anim.Config{Shape: []string{image}, Mask: []string{coloured}, AutoTrans: true})

	img, msk := rows(image), rows(mask)
	if len(msk) > len(img) {
		t.Errorf("%s: mask has %d rows, image only %d", name, len(msk), len(img))
	}
	for i := 0; i < min(len(img), len(msk)); i++ {
		if m := strings.TrimRight(msk[i], " "); len(m) > len(img[i]) {
			t.Errorf("%s row %d: mask %q is longer than image %q", name, i, msk[i], img[i])
		}
	}
}

func TestFishArtIsInPairs(t *testing.T) {
	for name, art := range map[string][]string{"new": newFishArt, "old": oldFishArt} {
		if len(art)%2 != 0 {
			t.Fatalf("%s fish art has an odd number of entries", name)
		}
		for i := 0; i < len(art); i += 2 {
			checkSprite(t, fmt.Sprintf("%s fish %d", name, i/2), art[i], art[i+1])
		}
	}
	if len(newFishArt) != 16 || len(oldFishArt) != 32 {
		t.Errorf("fish art has %d/%d entries, want 16/32", len(newFishArt), len(oldFishArt))
	}
}

func TestOtherArt(t *testing.T) {
	for dir := range 2 {
		checkSprite(t, fmt.Sprint("shark ", dir), sharkImage[dir], sharkMask[dir])
		checkSprite(t, fmt.Sprint("ship ", dir), shipImage[dir], shipMask[dir])
		checkSprite(t, fmt.Sprint("big fish 1 ", dir), bigFish1Image[dir], bigFish1Mask[dir])
		checkSprite(t, fmt.Sprint("big fish 2 ", dir), bigFish2Image[dir], bigFish2Mask[dir])
		for f, img := range newMonsterImage[dir] {
			checkSprite(t, fmt.Sprintf("new monster %d/%d", dir, f), img, newMonsterMask[dir])
		}
		for f, img := range oldMonsterImage[dir] {
			checkSprite(t, fmt.Sprintf("old monster %d/%d", dir, f), img, oldMonsterMask[dir])
		}
	}
	if len(newMonsterImage[0]) != 2 || len(newMonsterImage[1]) != 2 ||
		len(oldMonsterImage[0]) != 4 || len(oldMonsterImage[1]) != 4 {
		t.Error("unexpected number of monster frames")
	}
	checkSprite(t, "castle", castleImage, castleMask)
}

// The first spout frame shows a single drop of water. It must sit right above
// the blowhole, whichever way the whale faces.
func TestWhaleSpoutIsAboveTheBlowhole(t *testing.T) {
	for _, goesRight := range []bool{true, false} {
		var a *anim.Animation
		for seed := uint64(0); a == nil; seed++ {
			q, candidate := newTestAquarium(100, 30, Options{}, seed)
			q.addWhale()
			if whale := candidate.EntitiesOfType("")[0]; (whale.Delta.X > 0) == goesRight {
				a = candidate
				whale.X = 40 // well inside the screen
			}
		}
		for range 5 { // five frames without a spout, then the first spout frame
			a.Animate()
		}
		canvas := &recorder{t: t, w: 100, h: 30}
		a.Draw(canvas)

		colonsInRow := func(y int) (xs []int) {
			for pos, ch := range canvas.cells {
				if pos[1] == y && ch == ':' {
					xs = append(xs, pos[0])
				}
			}
			return xs
		}
		// Rows 0-2 hold the spout, row 3 is the top of the whale's head.
		spout, head := colonsInRow(2), colonsInRow(3)
		if len(spout) != 1 || len(head) != 1 || spout[0] != head[0] {
			t.Errorf("goesRight=%v: spout at %v, blowhole at %v", goesRight, spout, head)
		}
	}
}
