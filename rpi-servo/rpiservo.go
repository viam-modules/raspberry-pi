// Package rpiservo implements pi servo
package rpiservo

/*
	This driver contains various functionalities of a servo motor used in
	conjunction with a Raspberry Pi. The servo connects via a GPIO pin and
	uses the pi module's pigpio daemon library to control the servo motor.
	The servo pin will override the default pin configuration of of the pi
	module, including PWM frequency and width.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #cgo LDFLAGS: -lpigpio
// #include "../rpi/pi.h"
import "C"

import (
	"context"
	"fmt"
	"time"

	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("viam", "raspberry-pi", "rpi-servo")

var (
	holdTime                = 250000000 // 250ms in nanoseconds
	servoDefaultMaxRotation = 180
)

// init registers a pi servo based on pigpio.
func init() {
	resource.RegisterComponent(
		servo.API,
		Model,
		resource.Registration[servo.Servo, *ServoConfig]{
			Constructor: newPiServo,
		},
	)
}

func newPiServo(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (servo.Servo, error) {
	newConf, err := resource.NativeConfig[*ServoConfig](conf)
	if err != nil {
		return nil, err
	}

	if newConf.Pin == "" {
		return nil, errors.New("need pin for pi servo")
	}

	bcom, have := rpiutils.BroadcomPinFromHardwareLabel(newConf.Pin)
	if !have {
		return nil, errors.Errorf("no hw mapping for %s", newConf.Pin)
	}

	theServo := &piPigpioServo{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		pin:    C.uint(bcom),
		opMgr:  operation.NewSingleOperationManager(),
	}
	if newConf.Min > 0 {
		theServo.min = uint32(newConf.Min)
	}
	if newConf.Max > 0 {
		theServo.max = uint32(newConf.Max)
	}
	theServo.maxRotation = uint32(newConf.MaxRotation)
	if theServo.maxRotation == 0 {
		theServo.maxRotation = uint32(servoDefaultMaxRotation)
	}
	if theServo.maxRotation < theServo.min {
		return nil, errors.New("maxRotation is less than minimum")
	}
	if theServo.maxRotation < theServo.max {
		return nil, errors.New("maxRotation is less than maximum")
	}

	theServo.pinname = newConf.Pin

	// Start separate connection from board to pigpio daemon
	// Needs to be called before using other pigpio functions
	piID := C.custom_pigpio_start()
	// Set communication ID for servo
	theServo.piID = piID

	if newConf.StartPos == nil {
		setPos := C.set_servo_pulsewidth(theServo.piID, theServo.pin, C.uint(1500)) // a 1500ms pulsewidth positions the servo at 90 degrees
		errorCode := int(setPos)
		if errorCode != 0 {
			return nil, rpiutils.ConvertErrorCodeToMessage(errorCode, "gpioServo failed with")
		}
	} else {
		setPos := C.set_servo_pulsewidth(
			theservo.piID, theServo.pin,
			C.uint(angleToPulseWidth(int(*newConf.StartPos), int(theServo.maxRotation))),
		)
		errorCode := int(setPos)
		if errorCode != 0 {
			return nil, rpiutils.ConvertErrorCodeToMessage(errorCode, "gpioServo failed with")
		}
	}
	if newConf.HoldPos == nil || *newConf.HoldPos {
		theServo.holdPos = true
	} else {
		theServo.res = C.get_servo_pulsewidth(theServo.piID, theServo.pin)
		theServo.holdPos = false
		C.set_servo_pulsewidth(theServo.piID, theServo.pin, C.uint(0)) // disables servo
	}

	return theServo, nil

}

// piPigpioServo implements a servo.Servo using pigpio.
type piPigpioServo struct {
	resource.Named
	resource.AlwaysRebuild
	logger      logging.Logger
	pin         C.uint
	pinname     string
	res         C.int
	min, max    uint32
	opMgr       *operation.SingleOperationManager
	pulseWidth  int // pulsewidth value, 500-2500us is 0-180 degrees, 0 is off
	holdPos     bool
	maxRotation uint32
	piID        C.int
}

// Move moves the servo to the given angle (0-180 degrees)
// This will block until done or a new operation cancels this one
func (s *piPigpioServo) Move(ctx context.Context, angle uint32, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()

	if s.min > 0 && angle < s.min {
		angle = s.min
	}
	if s.max > 0 && angle > s.max {
		angle = s.max
	}
	pulseWidth := angleToPulseWidth(int(angle), int(s.maxRotation))
	res := C.set_servo_pulsewidth(s.piID, s.pin, C.uint(pulseWidth))

	s.pulseWidth = pulseWidth

	if res != 0 {
		err := s.pigpioErrors(int(res))
		return err
	}

	utils.SelectContextOrWait(ctx, time.Duration(pulseWidth)*time.Microsecond) // duration of pulswidth send on pin and servo moves

	if !s.holdPos { // the following logic disables a servo once it has reached a position or after a certain amount of time has been reached
		time.Sleep(time.Duration(holdTime)) // time before a stop is sent
		setPos := C.set_servo_pulsewidth(s.piID, s.pin, C.uint(0))
		if setPos < 0 {
			return errors.Errorf("servo on pin %s failed with code %d", s.pinname, setPos)
		}
	}
	return nil
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

// Position returns the current set angle (degrees) of the servo.
func (s *piPigpioServo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	res := C.get_servo_pulsewidth(s.piID, s.pin)
	err := s.pigpioErrors(int(res))
	if int(res) != 0 {
		s.res = res
	}
	if err != nil {
		return 0, err
	}
	return uint32(pulseWidthToAngle(int(s.res), int(s.maxRotation))), nil
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

// Stop stops the servo. It is assumed the servo stops immediately.
func (s *piPigpioServo) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, done := s.opMgr.New(ctx)
	defer done()
	getPos := C.set_servo_pulsewidth(s.piID, s.pin, C.uint(0))
	errorCode := int(getPos)
	if errorCode != 0 {
		return rpiutils.ConvertErrorCodeToMessage(errorCode, "gpioServo failed with")
	}
	return nil
}

func (s *piPigpioServo) IsMoving(ctx context.Context) (bool, error) {
	err := s.pigpioErrors(int(s.res))
	if err != nil {
		return false, err
	}
	if int(s.res) == 0 {
		return false, nil
	}
	return s.opMgr.OpRunning(), nil
}

// Close function to stop socket connection to pigpio daemon
func (s *piPigpioServo) Close(_ context.Context) error {
	C.custom_pigpio_stop(s.piID)

	return nil
}
