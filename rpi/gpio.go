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
	"fmt"

	rpiutils "raspberry-pi/utils"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	rdkutils "go.viam.com/rdk/utils"
)

// GPIOConfig tracks what each pin is currently configured as
type GPIOConfig int

const (
	GPIODefault   GPIOConfig = iota // GPIODefault is the default pin state, before we have modified the pin
	GPIOInput                       // GPIOInput is when a pin is configured as a digital input
	GPIOOutput                      // GPIOOutput is when the pin is configured as a digital output
	GPIOPWM                         // GPIOPWM is when the pin is configured as pwm
	GPIOInterrupt                   // GPIOInterrupt is the pin is configured as an interrupt
)

type rpiGPIO struct {
	name          string
	pin           uint
	configuration GPIOConfig
	pwmEnabled    bool
}

// GPIOPinByName returns a GPIOPin by name.
func (pi *piPigpio) GPIOPinByName(pin string) (board.GPIOPin, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	bcom, have := rpiutils.BroadcomPinFromHardwareLabel(pin)

	// check if we have already configured the pin
	for _, configuredPin := range pi.gpioPins {
		if configuredPin.name == pin {
			return gpioPin{pi, int(configuredPin.pin)}, nil
		}
		// check if the pin was configured with a different name
		if have && configuredPin.pin == bcom {
			pi.logger.Warnf("pin %v has already been configured with name %v", pin, configuredPin.name)
			return gpioPin{pi, int(configuredPin.pin)}, nil
		}
	}
	if !have {
		return nil, errors.Errorf("no hw pin for (%s)", pin)
	}

	// the pin was not found, so add a new pin to the map
	pi.gpioPins[int(bcom)] = &rpiGPIO{pin: bcom, name: pin}

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

func (pi *piPigpio) reconfigureGPIOs(ctx context.Context, cfg *Config) error {
	// Set new pins based on config
	pi.gpioPins = map[int]*rpiGPIO{}
	for _, newConfig := range cfg.Pins {
		if newConfig.Type != rpiutils.PinGPIO {
			continue
		}
		bcom, have := rpiutils.BroadcomPinFromHardwareLabel(newConfig.Pin)
		if !have {
			return errors.Errorf("no hw pin for (%s)", newConfig.Pin)
		}
		pin := &rpiGPIO{name: newConfig.Name, pin: bcom}
		pi.gpioPins[int(bcom)] = pin
	}
	return nil
}

// GetGPIOBcom gets the level of the given broadcom pin
func (pi *piPigpio) GetGPIOBcom(bcom int) (bool, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// verify we are currently managing this pin via GPIOPinByName or reconfigure
	pin, ok := pi.gpioPins[bcom]
	if !ok {
		return false, fmt.Errorf("error getting GPIO pin, pin %v not found", bcom)
	}
	// configure the pin to be an input if it is not already an input
	if pin.configuration != GPIOInput {
		res := C.set_mode(pi.piID, C.uint(pin.pin), C.PI_INPUT)
		if res != 0 {
			return false, rpiutils.ConvertErrorCodeToMessage(int(res), "failed to set mode")
		}
	}
	pin.configuration = GPIOInput

	// gpioRead retrns an int 1 or 0, we convert to a bool
	return C.gpio_read(pi.piID, C.uint(bcom)) != 0, nil
}

// SetGPIOBcom sets the given broadcom pin to high or low.
func (pi *piPigpio) SetGPIOBcom(bcom int, high bool) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// verify we are currently managing this pin via GPIOPinByName or reconfigure
	pin, ok := pi.gpioPins[bcom]
	if !ok {
		return fmt.Errorf("error getting GPIO pin, pin %v not found", bcom)
	}
	// configure the pin to be an output if it is not already an output
	if pin.configuration != GPIOOutput {
		// first if the pin was configured for pwm, we should turn off the pwm
		if pin.pwmEnabled {
			res := C.set_PWM_dutycycle(pi.piID, C.uint(pin.pin), C.uint(0))
			if res != 0 {
				return errors.Errorf("pwm set fail %d", res)
			}
			pin.pwmEnabled = false
		}
		res := C.set_mode(pi.piID, C.uint(pin.pin), C.PI_OUTPUT)
		if res != 0 {
			return rpiutils.ConvertErrorCodeToMessage(int(res), "failed to set mode")
		}
	}

	v := 0
	if high {
		v = 1
	}
	C.gpio_write(pi.piID, C.uint(pin.pin), C.uint(v))

	return nil
}

func (pi *piPigpio) pwmBcom(bcom int) (float64, error) {
	// verify we are currently managing this pin via GPIOPinByName or reconfigure
	pin, ok := pi.gpioPins[bcom]
	if !ok {
		return 0, fmt.Errorf("error getting GPIO pin, pin %v not found", bcom)
	}
	if !pin.pwmEnabled {
		pi.logger.Debugf("pin %v is currently not configured as pwm", bcom)
		return 0, nil
	}
	res := C.get_PWM_dutycycle(pi.piID, C.uint(pin.pin))
	return float64(res) / 255, nil
}

// SetPWMBcom sets the given broadcom pin to the given PWM duty cycle.
func (pi *piPigpio) SetPWMBcom(bcom int, dutyCyclePct float64) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pin, ok := pi.gpioPins[bcom]
	if !ok {
		return fmt.Errorf("error getting GPIO pin, pin %v not found", bcom)
	}

	dutyCycle := rdkutils.ScaleByPct(255, dutyCyclePct)
	res := C.set_PWM_dutycycle(pi.piID, C.uint(pin.pin), C.uint(dutyCycle))
	if res != 0 {
		return errors.Errorf("pwm set fail %d", res)
	}
	pin.configuration = GPIOPWM
	pin.pwmEnabled = true
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
