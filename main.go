package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
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

	// get chip
	chip, err := gpiocdev.NewChip(CHIP_NAME)
	if err != nil {
		log.Fatalf("unable to open %s: %v", CHIP_NAME, err)
	}
	defer chip.Close()

	// Initialize timing for PPM
	running := true
	ppm, err := NewPpmLineInput(chip, PIN_PPM, PPM_CHANNELCOUNT, PPM_SYNCTHRESHOLD)
	if err != nil {
		log.Fatalf("ppm failed to start: %v", err)
		os.Exit(1)
	}
	fmt.Printf("PPM read started on line %d...\n", PIN_PPM)

	// setup pwm outputs
	pwmLines = make([]*PwmLineOutput, MOTOR_COUNT)
	motor_pins := []int{
		PIN_ECS_1, // MOTOR_FRONT_LEFT
		PIN_ECS_2, // MOTOR_FRONT_RIGHT
		PIN_ECS_3, // MOTOR_BACK_LEFT
		PIN_ECS_4, // MOTOR_BACK_RIGHT
	}
	for i, v := range motor_pins {
		pwm, err := NewPwmLineOutput(chip, v, 1000, 0)
		if err != nil {
			log.Fatalf("unable to create PWM line on %d: %v", v, err)
			os.Exit(1)
		}
		pwmLines[i] = pwm
	}

	// start pwm signals
	for i, v := range pwmLines {
		wg.Add(1)
		go func() {
			v.Start()
			wg.Done()
		}()
		fmt.Printf("PWM Motor %d output started on line %d...\n", i, v.line.Offset())
	}

	// Capture SIGINT (Ctrl+C) to exit gracefully
	// Run until interrupted
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	connected := false
	ppmConnected := false
	for running {
		select {
		case <-sigChan:
			fmt.Println("\nStopping...")
			running = false
			connected = false
			ppmConnected = false
		default:
			// main loop

			if ppm.channels[CHAN_THROTTLE] < 900 || time.Now().Sub(ppm.lastTime) > 20*time.Millisecond {
				ppmConnected = false
			} else if ppm.channels[CHAN_THROTTLE] > 900 {
				ppmConnected = true
			}

			for _, v := range pwmLines {
				v.SetFrequency(int(ppm.channels[CHAN_THROTTLE]))
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
		v.Stop()
	}
	ppm.Stop()
	wg.Wait()

	fmt.Println("\nexiting motor controller...")
}
