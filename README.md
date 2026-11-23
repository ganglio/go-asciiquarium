# go-asciiquarium

An aquarium/sea animation in ASCII art for your terminal: a Go port of
[asciiquarium](http://robobunny.com/projects/asciiquarium) 1.1 by Kirk Baucom
([source](https://github.com/cmatsuoka/asciiquarium)).

Fish swim, bubbles rise, seaweed sways, and now and then a ship, a whale, a
sea monster, a big fish, a shark, a yellow submarine or a jellyfish passes by.
Sharks eat small fish.

## Run

```sh
go run .          # or: go build && ./go-asciiquarium
go run . -c       # "classic" mode: only the species of Asciiquarium 1.0
                  # (no newer fish and monster, no submarine, no jellyfish)
go run . -low     # low-capability mode: no colors, one fish, see below
go run . -version
```

While running:

| key | action                                               |
| --- | ---------------------------------------------------- |
| `q` | quit (so does Ctrl-C)                                |
| `r` | redraw: recreates everything and picks up a new terminal size |
| `p` | pause / resume                                       |
| `f` | drop food into the tank (low-capability mode only)   |

Unlike the Perl original it needs no Perl, Curses or CPAN modules, just a
terminal. It uses [tcell](https://github.com/gdamore/tcell).

## How it is put together

| path                | what                                                        |
| ------------------- | ----------------------------------------------------------- |
| `main.go`           | command line, terminal setup, key handling, 10 frames/second |
| `internal/anim`     | the animation engine: a port of the part of Perl's `Term::Animation` that asciiquarium uses (sprites with frames and color masks, depth ordering, collisions, the ways an entity can die). It draws to a `Canvas` interface and knows nothing about terminals. |
| `internal/aquarium` | the scene: the Go counterpart of the script's `add_*` functions. `art.go` holds the ASCII art. |

## Low-capability mode

`-low` is a bare-bones aquarium for limited terminals (not part of the
original). It uses no colors at all, only the terminal's own. The scene is
one line of water surface (`~`) at the top, the algae and bubbles of the
normal aquarium, and a single fish, `<º(((((()=<`.

The fish swims around the whole tank, not just from side to side, and turns
around instantly by flipping to its mirror image `>=())))))º>`, even in the
middle of a swim. Now and then it changes its mind and heads somewhere else.

Press `f` to feed it. Each press drops 5 to 10 pellets (`*`) from the middle
of the tank with a random spread. Every pellet sinks slowly at its own speed
and, once at the bottom, stays there. The fish swims to the closest pellet
and eats whatever it touches, also on the bottom. The tank holds at most 100
pellets. `f` does nothing in the normal aquarium.

## Beyond the original script

Two creatures come from open pull requests of the
[original repository](https://github.com/cmatsuoka/asciiquarium), which has not
merged them:

- the **yellow submarine** ([#6](https://github.com/cmatsuoka/asciiquarium/pull/6),
  Daniel Riechers): it cruises to the middle of the screen, raises its
  periscope, looks both ways, lowers it and carries on;
- the **jellyfish** ([#25](https://github.com/cmatsuoka/asciiquarium/pull/25),
  jkstill): it drifts sideways while slowly sinking.

Like the newer fish and sea monster they are left out in classic mode (`-c`).

## Differences from the Perl script

The behavior is meant to be the same. What differs:

- **Terminal library:** tcell instead of Curses, so it also works on Windows.
- **Frame timing:** a steady 10 frames per second. In the original every
  keypress also advanced a frame.
- **Bold colors** are drawn with the bright half of the ANSI palette instead
  of the bold attribute, so they look the same whatever the terminal font.
- **Resizing:** as in the original, a resized terminal is ignored until you
  press `r`.
- **Update order:** entities are updated in creation order; Perl visited them
  in random hash order. This only affects entities at the same depth.
- Not ported because the original never used them: the debug log, the unused
  signal handler and `center`.

## Tests

```sh
go test ./...          # about 15 s, mostly a long randomized run
go test -short ./...   # a shorter one
```

Besides unit tests of the engine and the scene, the suite runs the whole
aquarium for the equivalent of 20 minutes on screens from 200x60 down to
5x3, checking that nothing panics, nothing is drawn outside the screen and
the scene never runs out of creatures.

The ASCII art was extracted from the Perl script with Perl's own parser and
compared byte for byte with the Go strings.

## Credits and license

- Original program: Kirk Baucom, 2003.
- Yellow submarine: Daniel Riechers. Jellyfish: jkstill.
- ASCII art: Joan Stark.
- Extra species (backported from the Asciiquarium Android live wallpaper):
  Claudio Matsuoka.

This is a derivative work of the original and is distributed under the same
terms, the GNU General Public License, version 2 or (at your option) any
later version. See [LICENSE](LICENSE).
