package main

import "time"

const (
	CHIP_NAME = "gpiochip0"
)

const (
	PIN_PPM   = 70
	PIN_ECS_1 = 0
	PIN_ECS_2 = 0
	PIN_ECS_3 = 0
	PIN_ECS_4 = 0
)

const (
	PPM_SYNCTHRESHOLD = 3000 * time.Microsecond // Sync pulse > 3ms
	PPM_CHANNELCOUNT  = 8                       // Number of PPM channels
)
