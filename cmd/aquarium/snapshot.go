// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"go-asciiquarium/internal/app"
	"go-asciiquarium/internal/epd"
)

var (
	white = color.Gray{Y: math.MaxUint8}
	black = color.Gray{Y: 0}
)

// takeSnapshot draws the tank as the badge's screen would show it and writes
// it to a PNG file. If feed is set the fish is fed first; steps updates then
// run before the picture is taken, each one moving the tank on as one refresh
// of the screen does. If asleep is set it is night: the fish is gone, the
// picture is in negative, and each update is one picture of the night.
func takeSnapshot(tank *app.App, path string, scale int, feed, asleep bool, steps int) error {
	if scale < 1 {
		return fmt.Errorf("scale must be at least 1, got %d", scale)
	}
	if asleep {
		tank.Sleep()
	}
	if feed {
		tank.Feed()
	}
	for range steps {
		if asleep {
			tank.NightUpdate()
		} else {
			tank.Step()
		}
	}
	var frame epd.Frame
	tank.Render(&frame)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, picture(&frame, scale)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%dx%d pixels, the screen is %dx%d)\n", path, epd.Width*scale, epd.Height*scale, epd.Width, epd.Height)
	return nil
}

// picture turns a frame into an image, black on white, each pixel scale
// times as large.
func picture(f *epd.Frame, scale int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, epd.Width*scale, epd.Height*scale))
	for y := 0; y < epd.Height; y++ {
		for x := 0; x < epd.Width; x++ {
			c := white
			if f.Black(x, y) {
				c = black
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetGray(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	return img
}
