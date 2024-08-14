package rpi

/*
	gpio.go: Implements GPIO functionality on Raspberry Pi.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpiod_if2
import "C"

import (
	"context"
	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	rdkutils "go.viam.com/rdk/utils"
)

// GPIOPinByName returns a GPIOPin by name.
func (pi *piPigpio) GPIOPinByName(pin string) (board.GPIOPin, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	bcom, have := rpiutils.BroadcomPinFromHardwareLabel(pin)
	if !have {
		return nil, errors.Errorf("no hw pin for (%s)", pin)
	}
	return gpioPin{pi, int(bcom)}, nil
}

type gpioPin struct {
	pi   *piPigpio
	bcom int
}

func (gp gpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	return gp.pi.SetGPIOBcom(gp.bcom, high)
}

func (gp gpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return gp.pi.GetGPIOBcom(gp.bcom)
}

func (gp gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return gp.pi.pwmBcom(gp.bcom)
}

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return gp.pi.SetPWMBcom(gp.bcom, dutyCyclePct)
}

func (gp gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return gp.pi.pwmFreqBcom(gp.bcom)
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return gp.pi.SetPWMFreqBcom(gp.bcom, freqHz)
}

// GetGPIOBcom gets the level of the given broadcom pin
func (pi *piPigpio) GetGPIOBcom(bcom int) (bool, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	if !pi.gpioConfigSet[bcom] {
		if pi.gpioConfigSet == nil {
			pi.gpioConfigSet = map[int]bool{}
		}
		res := C.set_mode(pi.piID, C.uint(bcom), C.PI_INPUT)
		if res != 0 {
			return false, rpiutils.ConvertErrorCodeToMessage(int(res), "failed to set mode")
		}
		pi.gpioConfigSet[bcom] = true
	}

	// gpioRead retrns an int 1 or 0, we convert to a bool
	return C.gpio_read(pi.piID, C.uint(bcom)) != 0, nil
}

// SetGPIOBcom sets the given broadcom pin to high or low.
func (pi *piPigpio) SetGPIOBcom(bcom int, high bool) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	if !pi.gpioConfigSet[bcom] {
		if pi.gpioConfigSet == nil {
			pi.gpioConfigSet = map[int]bool{}
		}
		res := C.set_mode(pi.piID, C.uint(bcom), C.PI_OUTPUT)
		if res != 0 {
			return rpiutils.ConvertErrorCodeToMessage(int(res), "failed to set mode")
		}
		pi.gpioConfigSet[bcom] = true
	}

	v := 0
	if high {
		v = 1
	}
	C.gpio_write(pi.piID, C.uint(bcom), C.uint(v))
	return nil
}

func (pi *piPigpio) pwmBcom(bcom int) (float64, error) {
	res := C.get_PWM_dutycycle(pi.piID, C.uint(bcom))
	return float64(res) / 255, nil
}

// SetPWMBcom sets the given broadcom pin to the given PWM duty cycle.
func (pi *piPigpio) SetPWMBcom(bcom int, dutyCyclePct float64) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	dutyCycle := rdkutils.ScaleByPct(255, dutyCyclePct)
	pi.duty = int(C.set_PWM_dutycycle(pi.piID, C.uint(bcom), C.uint(dutyCycle)))
	if pi.duty != 0 {
		return errors.Errorf("pwm set fail %d", pi.duty)
	}
	return nil
}

func (pi *piPigpio) pwmFreqBcom(bcom int) (uint, error) {
	res := C.get_PWM_frequency(pi.piID, C.uint(bcom))
	return uint(res), nil
}

// SetPWMFreqBcom sets the given broadcom pin to the given PWM frequency.
func (pi *piPigpio) SetPWMFreqBcom(bcom int, freqHz uint) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	if freqHz == 0 {
		freqHz = 800 // Original default from libpigpio
	}
	newRes := C.set_PWM_frequency(pi.piID, C.uint(bcom), C.uint(freqHz))

	if newRes == C.PI_BAD_USER_GPIO {
		return rpiutils.ConvertErrorCodeToMessage(int(newRes), "pwm set freq failed")
	}

	if newRes != C.int(freqHz) {
		pi.logger.Infof("cannot set pwm freq to %d, setting to closest freq %d", freqHz, newRes)
	}
	return nil
}
