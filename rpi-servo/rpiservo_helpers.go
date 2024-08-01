package rpiservo

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "../rpi/pi.h"
import "C"

import (
	"fmt"
	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// Validate and set piPigpioServo fields based on the configuration.
func (s *piPigpioServo) validateAndSetConfiguration(conf *ServoConfig) error {
	if conf.Min >= 0 {
		s.min = uint32(conf.Min)
	}

	s.max = 180
	if conf.Max > 0 {
		s.max = uint32(conf.Max)
	}
	s.maxRotation = uint32(conf.MaxRotation)
	if s.maxRotation == 0 {
		s.maxRotation = uint32(servoDefaultMaxRotation)
	}
	if s.maxRotation < s.min {
		return errors.New("maxRotation is less than minimum")
	}
	if s.maxRotation < s.max {
		return errors.New("maxRotation is less than maximum")
	}
	if conf.Freq > 0 {
		s.pwmFreqHz = C.uint(conf.Freq)
	}

	s.pinname = conf.Pin

	return nil
}

// setInitialPosition sets the initial position of the servo based on the provided configuration.
func setInitialPosition(piServo *piPigpioServo, newConf *ServoConfig) error {
	position := 1500
	if newConf.StartPos != nil {
		position = angleToPulseWidth(int(*newConf.StartPos), int(piServo.maxRotation))
	}
	err := piServo.setServoPulseWidth(position)
	if err != nil {
		return err
	}
	return nil
}

// handleHoldPosition configures the hold position setting for the servo.
func handleHoldPosition(piServo *piPigpioServo, newConf *ServoConfig) error {
	if newConf.HoldPos == nil || *newConf.HoldPos {
		// Hold the servo position
		piServo.holdPos = true
	} else {
		// Release the servo position and disable the servo
		piServo.pwInUse = C.get_PWM_dutycycle(piServo.piID, piServo.pin)
		piServo.holdPos = false
		err := piServo.setServoPulseWidth(0)
		if err != nil {
			return fmt.Errorf("erroring setting pulse width to 0")
		}
	}
	return nil
}

// sets the servo's pulse width
func (s *piPigpioServo) setServoPulseWidth(pulseWidth int) error {
	errCode := C.set_PWM_frequency(s.piID, s.pin, s.pwmFreqHz)
	if errCode < 0 {
		return errors.Errorf("servo set pwm frequency on pin %s failed: %w", s.pinname, s.pigpioErrors(int(errCode)))
	}
	errCode = C.set_PWM_range(s.piID, s.pin, 1e6/s.pwmFreqHz)
	if errCode < 0 {
		return errors.Errorf("servo set pwm range on pin %s failed: %w", s.pinname, s.pigpioErrors(int(errCode)))
	}
	errCode = C.set_PWM_dutycycle(s.piID, s.pin, C.uint(pulseWidth))
	if errCode < 0 {
		return errors.Errorf("servo set pwm duty cycle on pin %s failed: %w", s.pinname, s.pigpioErrors(int(errCode)))
	}
	return nil
}

// parseConfig parses the provided configuration into a ServoConfig.
func parseConfig(conf resource.Config) (*ServoConfig, error) {
	newConf, err := resource.NativeConfig[*ServoConfig](conf)
	if err != nil {
		return nil, err
	}
	return newConf, nil
}

// validateConfig validates the provided ServoConfig.
func validateConfig(newConf *ServoConfig) error {
	if newConf.Pin == "" {
		return errors.New("need pin for pi servo")
	}
	return nil
}

// getBroadcomPin retrieves the Broadcom pin number from the hardware label.
func getBroadcomPin(pin string) (uint, error) {
	bcom, have := rpiutils.BroadcomPinFromHardwareLabel(pin)
	if !have {
		return 0, errors.Errorf("no hw mapping for %s", pin)
	}
	return bcom, nil
}

// pigpioErrors returns piGPIO specific errors to user
func (s *piPigpioServo) pigpioErrors(res int) error {
	switch {
	case res == C.PI_NOT_SERVO_GPIO:
		return errors.Errorf("gpioservo pin %s is not set up to send and receive pulsewidths", s.pinname)
	case res == C.PI_BAD_PULSEWIDTH:
		return errors.Errorf("gpioservo on pin %s trying to reach out of range position", s.pinname)
	case res == 0:
		return nil
	case res < 0 && res != C.PI_BAD_PULSEWIDTH && res != C.PI_NOT_SERVO_GPIO:
		errMsg := fmt.Sprintf("gpioServo on pin %s failed", s.pinname)
		return rpiutils.ConvertErrorCodeToMessage(res, errMsg)
	default:
		return nil
	}
}

// angleToPulseWidth changes the input angle in degrees
// into the corresponding pulsewidth value in microsecond
func angleToPulseWidth(angle, maxRotation int) int {
	pulseWidth := 500 + (2000 * angle / maxRotation)
	return pulseWidth
}

// pulseWidthToAngle changes the pulsewidth value in microsecond
// to the corresponding angle in degrees
func pulseWidthToAngle(pulseWidth, maxRotation int) int {
	angle := maxRotation * (pulseWidth + 1 - 500) / 2000
	return angle
}
