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
	lastTime time.Time
	ppmData  []time.Duration
	ppmReady bool
)

var kalmanFilters = make([]*KalmanFilter, PPM_CHANNELCOUNT)

func init() {
	// Initialize Kalman filters for each channel
	for i := 0; i < PPM_CHANNELCOUNT; i++ {
		kalmanFilters[i] = NewKalmanFilter(0.1, 10.0) // Example q and r values
	}
}

func ppmEventHandler(evt gpiocdev.LineEvent) {
	now := time.Now()
	pulseWidth := now.Sub(lastTime)
	lastTime = now

	if evt.Type == gpiocdev.LineEventRisingEdge {
		if pulseWidth > PPM_SYNCTHRESHOLD {
			// Sync pulse detected, process data if valid
			if len(ppmData) == PPM_CHANNELCOUNT {
				fmt.Print("\rChannels: ")
				for i, d := range ppmData {
					smoothedValue := kalmanFilters[i].Update(float64(d.Microseconds()))
					fmt.Printf("[%d]: %4.0fµs(%4dµs) ", i+1, smoothedValue, d.Microseconds())
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
	}
	defer l.Close()

	// Initialize timing
	lastTime = time.Now()

	// Capture SIGINT (Ctrl+C) to exit gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Printf("Listening for PPM signals on %s:%d...\n", CHIP_NAME, PIN_PPM)

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
