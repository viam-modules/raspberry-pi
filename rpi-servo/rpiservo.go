// Package rpiservo implements pi servo
package rpiservo

/*
	This driver contains various functionalities of a servo motor used in
	conjunction with a Raspberry Pi. The servo connects via a GPIO pin and
	uses the pi module's pigpio daemon library to control the servo motor.
	The servo pin will override the default pin configuration of of the pi
	module, including PWM frequency and width.

	Servo hardware model: DigiKey - SER0006 DFRobot
	https://www.digikey.com/en/products/detail/dfrobot/SER0006/7597224?WT.mc_id=frommaker.io

	Servo datasheet:
	http://www.ee.ic.ac.uk/pcheung/teaching/DE1_EE/stores/sg90_datasheet.pdf
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "../rpi/pi.h"
// #cgo LDFLAGS: -lpigpiod_if2
import "C"

import (
	"context"
	"time"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

var Model = resource.NewModel("viam-hardware-testing", "raspberry-pi", "rpi-servo")

// Default configuration collected from data sheet
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
	newConf, err := parseConfig(conf)
	if err != nil {
		return nil, err
	}

	if err := validateConfig(newConf); err != nil {
		return nil, err
	}

	bcom, err := getBroadcomPin(newConf.Pin)
	if err != nil {
		return nil, err
	}

	piServo, err := initializeServo(conf, logger, bcom, newConf)
	if err != nil {
		return nil, err
	}

	if err := setInitialPosition(piServo, newConf); err != nil {
		return nil, err
	}

	if err := handleHoldPosition(piServo, newConf); err != nil {
		return nil, err
	}

	return piServo, nil
}

// initializeServo creates and initializes the piPigpioServo with the provided configuration and logger.
func initializeServo(conf resource.Config, logger logging.Logger, bcom uint, newConf *ServoConfig) (*piPigpioServo, error) {
	piServo := &piPigpioServo{
		Named:     conf.ResourceName().AsNamed(),
		logger:    logger,
		pin:       C.uint(bcom),
		pinname:   newConf.Pin,
		opMgr:     operation.NewSingleOperationManager(),
		pwmFreqHz: 50, // default frequency for most pi hobby servos
	}

	piServo.logger.Infof("setting default pwm frequency of 50 Hz")

	if err := piServo.validateAndSetConfiguration(newConf); err != nil {
		return nil, err
	}

	// Start separate connection from board to pigpio daemon
	// Needs to be called before using other pigpio functions
	piID := C.pigpio_start(nil, nil)
	// Set communication ID for servo
	piServo.piID = piID

	return piServo, nil
}

// piPigpioServo implements a servo.Servo using pigpio.
type piPigpioServo struct {
	resource.Named
	resource.AlwaysRebuild
	logger      logging.Logger
	pin         C.uint
	pinname     string
	pwInUse     C.int // pulsewidth in use
	min, max    uint32
	opMgr       *operation.SingleOperationManager
	pulseWidth  int // pulsewidth value, 500-2500us is 0-180 degrees, 0 is off
	holdPos     bool
	maxRotation uint32
	piID        C.int
	pwmFreqHz   C.uint
}

// Move moves the servo to the given angle (0-180 degrees)
// This will block until done or a new operation cancels this one
func (s *piPigpioServo) Move(ctx context.Context, angle uint32, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()

	if s.min > 0 && angle < s.min {
		angle = s.min
		s.logger.Warnf("move angle %d is less than minimum %d, setting default to minimum angle", angle, s.min)
	}
	if s.max > 0 && angle > s.max {
		angle = s.max
		s.logger.Warnf("move angle %d is greater than maximum %d, setting default to maximum angle", angle, s.max)
	}
	pulseWidth := angleToPulseWidth(int(angle), int(s.maxRotation))
	err := s.setServoPulseWidth(pulseWidth)
	if err != nil {
		return err
	}

	s.pulseWidth = pulseWidth

	utils.SelectContextOrWait(ctx, time.Duration(pulseWidth)*time.Microsecond) // duration of pulsewidth send on pin and servo moves

	if !s.holdPos { // the following logic disables a servo once it has reached a position or after a certain amount of time has been reached
		time.Sleep(time.Duration(holdTime)) // time before a stop is sent
		err := s.setServoPulseWidth(pulseWidth)
		if err != nil {
			return err
		}
	}
	return nil
}

// Position returns the current set angle (degrees) of the servo.
func (s *piPigpioServo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	pwInUse := C.get_PWM_dutycycle(s.piID, s.pin)
	err := s.pigpioErrors(int(pwInUse))
	if int(pwInUse) != 0 {
		s.pwInUse = pwInUse
	}
	if err != nil {
		return 0, err
	}
	return uint32(pulseWidthToAngle(int(s.pwInUse), int(s.maxRotation))), nil
}

// Stop stops the servo. It is assumed the servo stops immediately.
func (s *piPigpioServo) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, done := s.opMgr.New(ctx)
	defer done()
	err := s.setServoPulseWidth(0)
	if err != nil {
		return err
	}
	return nil
}

// IsMoving returns whether the servo is actively moving (or attempting to move) under its own power.
func (s *piPigpioServo) IsMoving(ctx context.Context) (bool, error) {
	err := s.pigpioErrors(int(s.pwInUse))
	if err != nil {
		return false, err
	}
	if int(s.pwInUse) == 0 {
		return false, nil
	}
	return s.opMgr.OpRunning(), nil
}

// Close function to stop socket connection to pigpio daemon
func (s *piPigpioServo) Close(_ context.Context) error {
	C.pigpio_stop(s.piID)

	return nil
}
