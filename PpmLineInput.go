package main

import (
	"log"
	"syscall"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

type PpmLineInput struct {
	line           *gpiocdev.Line
	lastTime       time.Time
	sync_threshold time.Duration
	channelCount   int
	channels       []int
	rawPulsesIdx   int
	rawPulses      []time.Duration
	filters        []*KalmanFilter
}

func NewPpmLineInput(chip *gpiocdev.Chip, lineNum int, num_channels int, sync_threshold time.Duration) (*PpmLineInput, error) {
	ppm := &PpmLineInput{
		line:           nil,
		sync_threshold: sync_threshold,
		channelCount:   num_channels,
		channels:       make([]int, num_channels),
		rawPulsesIdx:   -1,
		rawPulses:      make([]time.Duration, num_channels),
		filters:        NewKalmanFilters(0.1, 10.0, num_channels),
	}

	line, err := gpiocdev.RequestLine(CHIP_NAME, PIN_PPM,
		gpiocdev.WithPullUp,
		gpiocdev.WithRisingEdge,
		gpiocdev.WithEventHandler(ppm.ppmEventHandler))
	if err != nil {
		log.Fatalf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			log.Fatalln("Note that the WithPullUp option requires Linux 5.5 or later - check your kernel version.")
		}
		return nil, err
	}

	ppm.line = line

	return ppm, nil
}

func (ppm *PpmLineInput) Stop() {
	ppm.line.Close()
}

func (ppm *PpmLineInput) FrameCallback() {
	// print filtered frame
	// fmt.Print("\r")
	// for i, v := range ppm.channels {
	// 	fmt.Printf("[%d]: %dµs ", i+1, v)
	// }
}

func (ppm *PpmLineInput) ppmEventHandler(evt gpiocdev.LineEvent) {
	now := time.Now()
	pulseWidth := now.Sub(ppm.lastTime)
	ppm.lastTime = now

	if evt.Type == gpiocdev.LineEventRisingEdge {
		if pulseWidth > ppm.sync_threshold || ppm.rawPulsesIdx == ppm.channelCount-1 {
			// Sync pulse detected, process data if valid
			if ppm.rawPulsesIdx == ppm.channelCount-1 {
				for i, d := range ppm.rawPulses {
					ppm.channels[i] = int(ppm.filters[i].Update(float64(d.Microseconds())))
				}
				ppm.FrameCallback()
			}
			// Reset for next frame
			ppm.rawPulsesIdx = -1
		} else {
			// Add pulse to current frame
			ppm.rawPulsesIdx += 1
			ppm.rawPulses[ppm.rawPulsesIdx] = pulseWidth
		}
	}
}
