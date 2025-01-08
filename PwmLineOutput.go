package main

import (
	"fmt"
	"os"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

type PwmLineOutput struct {
	cycle    time.Duration
	lastTime time.Time
	line     *gpiocdev.Line
	maxValue time.Duration
	minValue time.Duration
	output   bool
	value    time.Duration
}

// NewPwmLineOutput creates a new PWM Line Output instance
func NewPwmLineOutput(chip *gpiocdev.Chip, lineOffset int, cycle time.Duration) *PwmLineOutput {
	line, err := chip.RequestLine(lineOffset, gpiocdev.AsOutput(0))
	if err != nil {
		fmt.Printf("Error requesting line: %s\n", err)
		os.Exit(1)
	}

	return &PwmLineOutput{
		line:     line,
		value:    0,
		cycle:    cycle,
		minValue: 1000,
		maxValue: 2000,
	}
}

// SetValue sets the active duration of the PWM signal within its cycle
func (plo *PwmLineOutput) SetValue(v time.Duration) {
	plo.value = v
}

// SetValue sets the active duration of the PWM signal within its cycle
func (plo *PwmLineOutput) SetMinMax(min time.Duration, max time.Duration) {
	plo.minValue = min
	plo.maxValue = max
}

// StopOutput stops the PWM signal output
func (plo *PwmLineOutput) StopOutput() {
	plo.output = false
}

// OutputPwm starts generating the PWM signal
func (plo *PwmLineOutput) OutputPwm() {
	plo.lastTime = time.Now()
	plo.output = true

	for plo.output {
		now := time.Now()
		diff := now.Sub(plo.lastTime)

		switch {
		case plo.value == 0:
			plo.line.SetValue(0)
		case diff >= plo.cycle:
			// Start new cycle
			plo.line.SetValue(1)
			plo.lastTime = now
		case diff >= plo.value:
			plo.line.SetValue(0)
		}
	}

	plo.line.Reconfigure(gpiocdev.AsInput)
	plo.line.Close()
}
