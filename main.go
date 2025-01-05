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

const smoothFactor = 0.1 // Smoothing factor (0.1 for a low-pass filter effect)

var lastSmoothedPulse time.Duration

var lastSmoothedPulses = make([]time.Duration, channelCount) // Store smoothed pulses for each channel

// Smooths the pulse width using a low-pass filter (exponential moving average)
func lowPassFilter(channel int, pulseWidth time.Duration) time.Duration {
	if lastSmoothedPulses[channel] == 0 {
		lastSmoothedPulses[channel] = pulseWidth
	}
	smoothed := time.Duration(float64(lastSmoothedPulses[channel]) + smoothFactor*float64(pulseWidth-lastSmoothedPulses[channel]))
	lastSmoothedPulses[channel] = smoothed
	return smoothed
}

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
					smoothedValue := lowPassFilter(i, d)
					fmt.Printf("[%d]: %4dµs(%4dµs) ", i+1, smoothedValue.Microseconds(), d.Microseconds())
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
				//fmt.Print("\rPPM frame processed.")
				ppmReady = false
			} else {
				fmt.Print("\rListening for PPM signals...\n")
			}
		}
	}

	fmt.Println("\nPPM reader exiting...")
}
