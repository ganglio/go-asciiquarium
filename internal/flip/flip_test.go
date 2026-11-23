// SPDX-License-Identifier: GPL-2.0-or-later

package flip

import "testing"

func TestString(t *testing.T) {
	for in, want := range map[string]string{
		"<))o":    "o((>",
		"<[{(":    ")}]>",
		"a":       "a",
		"=":       "=",
		"":        "",
		"<º(((=<": ">=)))º>",
	} {
		if got := String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
		if got := String(String(in)); got != in {
			t.Errorf("mirroring %q twice gives %q", in, got)
		}
	}
}

func TestRune(t *testing.T) {
	for _, pair := range Pairs {
		if Rune(pair[0]) != pair[1] || Rune(pair[1]) != pair[0] {
			t.Errorf("%q and %q do not turn into each other", pair[0], pair[1])
		}
	}
	if Rune('o') != 'o' {
		t.Error("'o' should look the same from both sides")
	}
}

func TestEveryCharacterIsInOnePairOnly(t *testing.T) {
	seen := map[rune]bool{}
	for _, pair := range Pairs {
		for _, r := range pair {
			if seen[r] {
				t.Errorf("%q is in more than one pair", r)
			}
			seen[r] = true
		}
		if pair[0] == pair[1] {
			t.Errorf("%q is paired with itself", pair[0])
		}
	}
}
