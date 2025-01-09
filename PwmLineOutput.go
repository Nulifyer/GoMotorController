package main

import (
	"time"

	"github.com/warthog618/go-gpiocdev"
)

type PwmLineOutput struct {
	dutyCycle time.Duration
	highTime  time.Duration
	line      *gpiocdev.Line
	lowTime   time.Duration
	stopChan  chan struct{}
}

// NewPwmLineOutput initializes a PwmLineOutput
func NewPwmLineOutput(chip *gpiocdev.Chip, lineNum int, frequency int, dutyCycle time.Duration) (*PwmLineOutput, error) {
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
	pwm.line.SetValue(0)
	return pwm, nil
}

func (pwm *PwmLineOutput) SetFrequency(f int) {
	frequency := time.Duration(f) * time.Microsecond
	pwm.highTime = frequency
	if pwm.highTime > pwm.dutyCycle {
		pwm.highTime = pwm.dutyCycle
	}

	pwm.lowTime = time.Duration(pwm.dutyCycle) - pwm.highTime
	if pwm.lowTime < 0 {
		pwm.lowTime = 0
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
			if pwm.highTime > 0 {
				pwm.line.SetValue(1)
				time.Sleep(pwm.highTime)
			}
			if pwm.lowTime > 0 {
				pwm.line.SetValue(0)
				time.Sleep(pwm.lowTime)
			}
			if pwm.highTime == 0 && pwm.lowTime == 0 {
				time.Sleep(10 * time.Microsecond)
			}
		}
	}
}
