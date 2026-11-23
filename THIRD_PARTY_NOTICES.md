# Third-party notices

## Pimoroni Badger 2350 firmware

The display driver in `internal/epd/driver.go` (the register sequence, the
waveform table and the layout of the picture in the controller's RAM) is
transcribed from `modules/c/ssd1680/ssd1680.cpp` in
<https://github.com/pimoroni/badger2350>, which is released under the MIT
license:

```
MIT License

Copyright (c) 2026 Pimoroni Ltd

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

The pin assignments in `cmd/badger/hardware.go` and the way the buttons and
the power pin are used follow the same repository (`board/pins.csv`,
`modules/c/powman/powman.c`) and TinyGo's board file for the badge.

## tcell

The program that runs on a computer (`cmd/aquarium`) uses
[tcell](https://github.com/gdamore/tcell) (Apache License 2.0) to draw in the
terminal. It is a dependency, not part of this repository, and the firmware
does not use it.
