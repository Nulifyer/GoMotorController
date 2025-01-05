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
	syncThreshold = 3000 * time.Microsecond // Sync pulse > 3ms
	channelCount  = 8                       // Number of PPM channels
)

var (
	lastTime time.Time
	ppmData  []time.Duration
	ppmReady bool
)

// Handles GPIO events and decodes PPM signals.
func ppmEventHandler(evt gpiocdev.LineEvent) {
	now := time.Now()
	pulseWidth := now.Sub(lastTime)
	lastTime = now

	if evt.Type == gpiocdev.LineEventRisingEdge {
		if pulseWidth > syncThreshold {
			// Sync pulse detected, process data if valid
			if len(ppmData) == channelCount {
				fmt.Print("\rChannels: ")
				for i, d := range ppmData {
					fmt.Printf("[%d]: %4dµs ", i+1, d.Microseconds())
				}
				ppmReady = true
			}
			// Reset for next frame
			ppmData = nil
		} else {
			// Add pulse to current frame
			ppmData = append(ppmData, pulseWidth)
		}
	}
}

func main() {
	offset := 70
	chip := "gpiochip0"

	// Request GPIO line with event handler for rising edges
	l, err := gpiocdev.RequestLine(chip, offset,
		gpiocdev.WithPullUp,
		gpiocdev.WithRisingEdge,
		gpiocdev.WithEventHandler(ppmEventHandler))
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

	fmt.Printf("Listening for PPM signals on %s:%d...\n", chip, offset)

	// Run until interrupted
	running := true
	for running {
		select {
		case <-sigChan:
			running = false
		default:
			time.Sleep(100 * time.Millisecond)
			if ppmReady {
				fmt.Print("\rPPM frame processed.")
				ppmReady = false
			} else {
				fmt.Print("\rListening for PPM signals...")
			}
		}
	}

	fmt.Println("\nPPM reader exiting...")
}
