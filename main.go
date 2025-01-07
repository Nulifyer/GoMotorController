package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// chip & pin numbers
const (
	CHIP_NAME = "gpiochip0"

	PIN_PPM   = 70
	PIN_ECS_1 = 0
	PIN_ECS_2 = 0
	PIN_ECS_3 = 0
	PIN_ECS_4 = 0
)

// ppm settings
const (
	PPM_SYNCTHRESHOLD       = 3000 * time.Microsecond
	PPM_CHANNELCOUNT        = 8
	PPM_CONNECTED_THRESHOLD = 900
)

// channel names
const (
	CHAN_STICK_RIGHT_X = 0
	CHAN_STICK_RIGHT_Y = 1
	CHAN_STICK_LEFT_Y  = 2
	CHAN_STICK_LEFT_X  = 3
	CHAN_SWITCH_RIGHT  = 4
	CHAN_DIAL_RIGHT    = 5
	CHAN_SWITCH_LEFT   = 6
	CHAN_DIAL_LEFT     = 7

	CHAN_THROTTLE = CHAN_STICK_LEFT_Y
)

// PPM variables
var (
	lastTime     time.Time
	rawPPMData   []time.Duration
	ppmConnected bool
	filters      = NewKalmanFilters(0.1, 10.0, PPM_CHANNELCOUNT)
)

func main() {
	// Initialize timing for PPM
	lastTime = time.Now()
	ppmConnected = false
	// Request GPIO line with event handler for rising edges
	l, err := gpiocdev.RequestLine(CHIP_NAME, PIN_PPM,
		gpiocdev.WithPullUp,
		gpiocdev.WithRisingEdge,
		gpiocdev.WithEventHandler(ppmEventHandler))
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires Linux 5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	} else {
		fmt.Printf("Listening for PPM signals on %s:%d...\n", CHIP_NAME, PIN_PPM)
	}
	defer l.Close()

	// Capture SIGINT (Ctrl+C) to exit gracefully
	// Run until interrupted
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	running := true
	connected := false
	for running {
		select {
		case <-sigChan:
			running = false
		default:
			// main loop

			if ppmConnected && time.Now().Sub(lastTime) > PPM_SYNCTHRESHOLD*10 {
				ppmConnected = false
			}

			if ppmConnected && !connected {
				connected = true
				fmt.Println("\nConnected...")
			} else if !ppmConnected && connected {
				connected = false
				fmt.Println("\nDisconnected...")
			}
		}
	}

	fmt.Println("\nexiting motor controller...")
}

func ppmFrameCallback(frame []float64) {
	// print filtered frame
	fmt.Print("\r")
	for i, v := range frame {
		fmt.Printf("[%d]: %4.0fµs ", i+1, v)
	}

	ppmConnected = frame[CHAN_THROTTLE] > PPM_CONNECTED_THRESHOLD
}

// processes line events into the PPM frame
func ppmEventHandler(evt gpiocdev.LineEvent) {
	now := time.Now()
	pulseWidth := now.Sub(lastTime)
	lastTime = now

	if evt.Type == gpiocdev.LineEventRisingEdge {
		if pulseWidth > PPM_SYNCTHRESHOLD {
			// Sync pulse detected, process data if valid
			if len(rawPPMData) == PPM_CHANNELCOUNT {
				filteredValues := make([]float64, PPM_CHANNELCOUNT)
				for i, d := range rawPPMData {
					filteredValues[i] = filters[i].Update(float64(d.Microseconds()))
				}
				ppmFrameCallback(filteredValues)
			}
			// Reset for next frame
			rawPPMData = nil
		} else {
			// Add pulse to current frame
			rawPPMData = append(rawPPMData, pulseWidth)
		}
	}
}
