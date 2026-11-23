// SPDX-License-Identifier: GPL-2.0-or-later

//go:build !badger2350

package main

import "fmt"

// This file keeps "go build ./..." and "go vet ./..." working on a computer.
// The firmware itself only builds for the badge.
func main() {
	fmt.Println("This is the firmware for the Pimoroni Badger 2350.")
	fmt.Println("Build it with: tinygo build -target=badger2350 -o bin/aquarium.uf2 ./cmd/badger   (or: make badger)")
	fmt.Println("To try the aquarium on this computer: go run ./cmd/aquarium   (or: make run)")
}
