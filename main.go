package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// chip & pin numbers
const (
	PC9  = 73
	PC6  = 70
	PC5  = 69
	PC8  = 72
	PC11 = 75
	PC15 = 79
	PC14 = 78
	PC7  = 71
	PC10 = 74
)
const (
	CHIP_NAME = "gpiochip0"

	PIN_PPM   = PC6
	PIN_ECS_1 = PC5
	PIN_ECS_2 = PC8
	PIN_ECS_3 = PC11
	PIN_ECS_4 = PC15
)

// ppm settings
const (
	PPM_SYNCTHRESHOLD       = 3 * time.Millisecond
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

// motor pwm controls
const (
	MOTOR_COUNT = 4
	PWM_CYCLE   = 3 * time.Millisecond

	MOTOR_FRONT_LEFT  = 0
	MOTOR_FRONT_RIGHT = 1
	MOTOR_BACK_LEFT   = 2
	MOTOR_BACK_RIGHT  = 3
)

var (
	pwmLines []*PwmLineOutput
)

func main() {
	var wg sync.WaitGroup

	// Initialize timing for PPM
	lastTime = time.Now()
	ppmConnected = false
	running := true

	wg.Add(1)
	go func() {
		// Request GPIO line with event handler for rising edges
		ppm_line, err := gpiocdev.RequestLine(CHIP_NAME, PIN_PPM,
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
		for running {
		}
		defer ppm_line.Close()
		defer wg.Done()
	}()

	// setup pwm outputs
	chip, err := gpiocdev.NewChip(CHIP_NAME)
	if err != nil {
		fmt.Printf("NewChip returned error: %s\n", err)
		os.Exit(1)
	}

	pwmLines = make([]*PwmLineOutput, MOTOR_COUNT)
	pwmLines[MOTOR_FRONT_LEFT] = NewPwmLineOutput(chip, PIN_ECS_1, PWM_CYCLE)
	fmt.Printf("PWM Motor FRONT_LEFT output on %s:%d...\n", CHIP_NAME, PIN_ECS_1)

	pwmLines[MOTOR_FRONT_RIGHT] = NewPwmLineOutput(chip, PIN_ECS_2, PWM_CYCLE)
	fmt.Printf("PWM Motor FRONT_RIGHT output on %s:%d...\n", CHIP_NAME, PIN_ECS_2)

	pwmLines[MOTOR_BACK_LEFT] = NewPwmLineOutput(chip, PIN_ECS_3, PWM_CYCLE)
	fmt.Printf("PWM Motor BACK_LEFT output on %s:%d...\n", CHIP_NAME, PIN_ECS_3)

	pwmLines[MOTOR_BACK_RIGHT] = NewPwmLineOutput(chip, PIN_ECS_4, PWM_CYCLE)
	fmt.Printf("PWM Motor BACK_RIGHT output on %s:%d...\n", CHIP_NAME, PIN_ECS_4)

	// start pwm signals
	for i, v := range pwmLines {
		wg.Add(1)
		go func() {
			v.OutputPwm()
			wg.Done()
		}()
		fmt.Printf("PWM Motor %d output started...\n", i)
	}

	// Capture SIGINT (Ctrl+C) to exit gracefully
	// Run until interrupted
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
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

	// signal to stop and clean up
	for _, v := range pwmLines {
		v.StopOutput()
	}
	wg.Wait()

	fmt.Println("\nexiting motor controller...")
}

func ppmFrameCallback(frame []float64) {
	// print filtered frame
	fmt.Print("\r")
	for i, v := range frame {
		fmt.Printf("[%d]: %4.0fµs ", i+1, v)
	}

	ppmConnected = frame[CHAN_THROTTLE] > PPM_CONNECTED_THRESHOLD

	for _, v := range pwmLines {
		v.SetValue(time.Duration(frame[CHAN_THROTTLE]) * time.Microsecond)
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
