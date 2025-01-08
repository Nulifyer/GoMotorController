package main

import (
	"time"

	"github.com/warthog618/go-gpiocdev"
)

type PwmLineOutput struct {
	line      *gpiocdev.Line
	frequency int
	dutyCycle int
	stopChan  chan struct{}

	highTime time.Duration
	lowTime  time.Duration
}

// NewPwmLineOutput initializes a PwmLineOutput
func NewPwmLineOutput(chip *gpiocdev.Chip, lineNum int, frequency, dutyCycle int) (*PwmLineOutput, error) {
	line, err := chip.RequestLine(lineNum, gpiocdev.AsOutput(0))
	if err != nil {
		return nil, err
	}
	pwm := &PwmLineOutput{
		line:      line,
		dutyCycle: dutyCycle,
		stopChan:  make(chan struct{}),
	}
	pwm.SetFrequency(frequency)
	return pwm, nil
}

func (pwm *PwmLineOutput) SetFrequency(f int) {
	pwm.frequency = f
	if pwm.frequency == 0 {
		pwm.highTime = 0
		pwm.lowTime = time.Second
	} else {
		period := time.Second / time.Duration(pwm.frequency)
		pwm.highTime = period * time.Duration(pwm.dutyCycle) / 100
		pwm.lowTime = period - pwm.highTime
	}
}

// Stop halts the PWM loop
func (pwm *PwmLineOutput) Stop() {
	close(pwm.stopChan)
	pwm.line.Reconfigure(gpiocdev.AsInput)
	pwm.line.Close()
}

// Start begins the PWM loop
func (pwm *PwmLineOutput) Start() {
	for {
		select {
		case <-pwm.stopChan:
			return
		default:
			pwm.line.SetValue(1)
			time.Sleep(pwm.highTime)
			pwm.line.SetValue(0)
			time.Sleep(pwm.lowTime)
		}
	}
}
