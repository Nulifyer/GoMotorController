package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

var (
	lastTime     time.Time
	rawPPMData   []time.Duration
	ppmConnected bool
	filters      = NewKalmanFilters(0.1, 10.0, PPM_CHANNELCOUNT)
)

func main() {
	// Initialize timing
	lastTime = time.Now()

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
	for running {
		select {
		case <-sigChan:
			running = false
		default:
			time.Sleep(100 * time.Millisecond)
			if ppmConnected {
				//fmt.Print("\rPPM frame processed.")
				ppmConnected = false
			} else {
				fmt.Print("\rListening for PPM signals...\n")
			}
		}
	}

	fmt.Println("\nPPM reader exiting...")
}

func ppmFrameCallback(frame *[]float64) {
	// print filtered frame
	fmt.Print("\r")
	for i, v := range *frame {
		fmt.Printf("[%d]: %4.0fµs ", i+1, v)
	}
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
				ppmFrameCallback(&filteredValues)
				ppmConnected = true
			}
			// Reset for next frame
			rawPPMData = nil
		} else {
			// Add pulse to current frame
			rawPPMData = append(rawPPMData, pulseWidth)
		}
	}
}
