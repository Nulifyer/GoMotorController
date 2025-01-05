// SPDX-FileCopyrightText: 2020 Kent Gibson <warthog618@gmail.com>
//
// SPDX-License-Identifier: MIT

//go:build linux

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

const (
	gpioPin     = 70                     // GPIO pin for SBUS signal
	frameSize   = 25                     // SBUS frame size (25 bytes)
	bitDuration = 100 * time.Microsecond // Timeout to sync reading SBUS frames

)

var (
	lastTime     time.Time
	sbusFrame    [frameSize]byte
	sbusBitIndex int
	sbusReady    bool
)

// Handles GPIO events and decodes SBUS signals.
func sbusEventHandler(evt gpiocdev.LineEvent) {
	now := time.Now()
	pulseWidth := now.Sub(lastTime)
	lastTime = now

	// If the pulse width is long enough to be considered a bit
	if evt.Type == gpiocdev.LineEventRisingEdge {
		// If we're currently within an SBUS frame, accumulate bits
		if sbusBitIndex < frameSize*8 {
			// Determine if this is a high or low bit based on pulse width
			if pulseWidth > bitDuration {
				// High bit (1)
				sbusFrame[sbusBitIndex/8] |= (1 << (7 - sbusBitIndex%8))
			} else {
				// Low bit (0)
				// No change to sbusFrame because it's already 0
			}

			// Move to the next bit in the frame
			sbusBitIndex++

			// If we've filled up all 25 bytes (200 bits), mark the frame as ready
			if sbusBitIndex == frameSize*8 {
				sbusReady = true
			}
		}
	}
}

func main() {
	chip := "gpiochip0" // The GPIO chip
	offset := gpioPin   // The GPIO pin offset for SBUS input

	// Request GPIO line with event handler for rising edges
	l, err := gpiocdev.RequestLine(chip, offset,
		gpiocdev.WithPullUp,
		gpiocdev.WithRisingEdge,
		gpiocdev.WithEventHandler(sbusEventHandler))
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires Linux 5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	}
	defer l.Close()

	// Initialize timing
	lastTime = time.Now()

	// Capture SIGINT (Ctrl+C) to exit gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Printf("Listening for SBUS signals on %s:%d...\n", chip, offset)

	// Run until interrupted
	running := true
	for running {
		select {
		case <-sigChan:
			running = false
		default:
			time.Sleep(100 * time.Millisecond)

			// If a complete SBUS frame is ready, process it
			if sbusReady {
				// Print the raw SBUS frame in hexadecimal format
				fmt.Print("\rSBUS frame processed: ")
				for _, byteVal := range sbusFrame {
					fmt.Printf("%02X ", byteVal)
				}
				fmt.Println()

				// Process the SBUS frame to extract channels, flags, etc.
				// Here, you would call a function to decode the channels

				// Reset for the next frame
				sbusReady = false
				sbusBitIndex = 0
			} else {
				fmt.Print("\rListening for SBUS signals...")
			}
		}
	}

	fmt.Println("\nSBUS reader exiting...")
}
