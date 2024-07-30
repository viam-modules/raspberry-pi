package rpiservo

// #include <stdlib.h>
// #include <pigpiod_if2.h>
import "C"
import (
	"context"
	"testing"
	"viamrpi/rpi"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
)

func TestPiServo(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("servo initialize with pin error", func(t *testing.T) {
		servoReg, ok := resource.LookupRegistration(servo.API, Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		_, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &ServoConfig{Pin: ""},
			},
			logger,
		)
		test.That(t, err.Error(), test.ShouldContainSubstring, "need pin for pi servo")
	})

	t.Run("check new servo defaults", func(t *testing.T) {
		ctx := context.Background()
		servoReg, ok := resource.LookupRegistration(servo.API, Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &ServoConfig{Pin: "22"},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		servo1 := servoInt.(servo.Servo)
		pos1, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos1, test.ShouldEqual, 90)
	})

	t.Run("check set default position", func(t *testing.T) {
		ctx := context.Background()
		servoReg, ok := resource.LookupRegistration(servo.API, Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)

		initPos := 33.0
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &ServoConfig{Pin: "22", StartPos: &initPos},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		servo1 := servoInt.(servo.Servo)
		pos1, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos1, test.ShouldEqual, 33)

		localServo := servo1.(*piPigpioServo)
		test.That(t, localServo.holdPos, test.ShouldBeTrue)
	})
}

func TestInitializationFunctions(t *testing.T) {
	t.Run("test setting initial position", func(t *testing.T) {
		s := &piPigpioServo{
			pinname:     "3",
			maxRotation: 180,
			opMgr:       operation.NewSingleOperationManager(),
			piID:        C.pigpio_start(nil, nil),
		}

		err := setInitialPosition(s, &ServoConfig{StartPos: nil})
		test.That(t, err, test.ShouldBeNil)

		initPos := 33.0
		err = setInitialPosition(s, &ServoConfig{StartPos: &initPos})
		test.That(t, err, test.ShouldBeNil)

		// invalid pin
		s.pinname = "bad"
		err = setInitialPosition(s, &ServoConfig{StartPos: nil})
		test.That(t, err.Error(), test.ShouldContainSubstring, "PI_BAD_GPIO")

		// invalid angle
		s.pinname = "3"
		initPos = 181.0
		err = setInitialPosition(s, &ServoConfig{StartPos: &initPos})
		test.That(t, err.Error(), test.ShouldContainSubstring, "PI_BAD_PULSEWIDTH")

		C.pigpio_stop(s.piID)
	})

	t.Run("test handle hold position", func(t *testing.T) {
		s := &piPigpioServo{
			pinname:     "3",
			maxRotation: 180,
			opMgr:       operation.NewSingleOperationManager(),
			piID:        C.pigpio_start(nil, nil),
		}

		handleHoldPosition(s, &ServoConfig{HoldPos: nil})
		test.That(t, s.holdPos, test.ShouldBeTrue)

		holdPos := true
		handleHoldPosition(s, &ServoConfig{HoldPos: &holdPos})
		test.That(t, s.holdPos, test.ShouldBeTrue)

		holdPos = false
		handleHoldPosition(s, &ServoConfig{HoldPos: &holdPos})
		test.That(t, s.holdPos, test.ShouldBeFalse)

		C.pigpio_stop(s.piID)
	})

	t.Run("test servo initialization", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		bcom := uint(3)
		conf := resource.Config{
			Name: "servo",
		}

		// invalid conf, maxRotation < min
		newConf := &ServoConfig{
			Pin:         "3",
			MaxRotation: 180,
			Min:         200,
		}

		s, err := initializeServo(conf, logger, bcom, newConf)
		test.That(t, s, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "maxRotation is less than minimum")

		// invalid conf, maxRotation < max
		newConf = &ServoConfig{
			Pin:         "3",
			Max:         180,
			MaxRotation: 179,
		}

		s, err = initializeServo(conf, logger, bcom, newConf)
		test.That(t, s, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "maxRotation is less than maximum")

		// valid conf
		newConf = &ServoConfig{
			Pin: "3",
		}

		targetPin := C.uint(3)

		s, err = initializeServo(conf, logger, bcom, newConf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, s, test.ShouldNotBeNil)
		test.That(t, int(s.piID), test.ShouldBeGreaterThanOrEqualTo, 0)
		test.That(t, s.pin, test.ShouldEqual, targetPin)

		// stop pigpio
		C.pigpio_stop(s.piID)
	})

}
func TestServoFunctions(t *testing.T) {
	t.Run("test parse config", func(t *testing.T) {
		newConf := &ServoConfig{Pin: "100"}

		parsedConf, err := parseConfig(
			resource.Config{ConvertedAttributes: newConf},
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, parsedConf, test.ShouldNotBeNil)
		test.That(t, parsedConf.Pin, test.ShouldEqual, "100")

		badConf := &rpi.Config{}
		parsedConf, err = parseConfig(
			resource.Config{ConvertedAttributes: badConf},
		)

		test.That(t, parsedConf, test.ShouldBeNil)
		// unexpected type, only kind of error
		test.That(t, err, test.ShouldNotBeNil)

	})
	t.Run("test config validation", func(t *testing.T) {
		newConf := &ServoConfig{Pin: "22"}
		err := validateConfig(newConf)
		test.That(t, err, test.ShouldBeNil)

		newConf = &ServoConfig{Pin: ""}
		err = validateConfig(newConf)
		test.That(t, err.Error(), test.ShouldContainSubstring, "need pin for pi servo")
	})

	t.Run("test get broadcom pin", func(t *testing.T) {
		// pin with special name/function
		bcom, err := getBroadcomPin("sclk")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bcom, test.ShouldEqual, 11)

		// standard pin
		bcom, err = getBroadcomPin("22")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bcom, test.ShouldEqual, 25)

		// pin based on IO
		bcom, err = getBroadcomPin("io21")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bcom, test.ShouldEqual, 21)

		// bad pin
		bcom, err = getBroadcomPin("bad")
		test.That(t, err.Error(), test.ShouldContainSubstring, "no hw mapping for bad")
		test.That(t, bcom, test.ShouldEqual, 0)
	})

	t.Run("check servo math", func(t *testing.T) {
		pw := angleToPulseWidth(1, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 511)
		pw = angleToPulseWidth(0, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 500)
		pw = angleToPulseWidth(179, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 2488)
		pw = angleToPulseWidth(180, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 2500)
		pw = angleToPulseWidth(179, 270)
		test.That(t, pw, test.ShouldEqual, 1825)
		pw = angleToPulseWidth(180, 270)
		test.That(t, pw, test.ShouldEqual, 1833)
		a := pulseWidthToAngle(511, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 1)
		a = pulseWidthToAngle(500, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 0)
		a = pulseWidthToAngle(2500, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 180)
		a = pulseWidthToAngle(2488, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 179)
		a = pulseWidthToAngle(1825, 270)
		test.That(t, a, test.ShouldEqual, 179)
		a = pulseWidthToAngle(1833, 270)
		test.That(t, a, test.ShouldEqual, 180)
	})

	t.Run(("check Move IsMoving and pigpio errors"), func(t *testing.T) {
		ctx := context.Background()
		s := &piPigpioServo{pinname: "1", maxRotation: 180, opMgr: operation.NewSingleOperationManager()}

		s.res = -93
		err := s.pigpioErrors(int(s.res))
		test.That(t, err.Error(), test.ShouldContainSubstring, "pulsewidths")
		moving, err := s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		s.res = -7
		err = s.pigpioErrors(int(s.res))
		test.That(t, err.Error(), test.ShouldContainSubstring, "range")
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		s.res = 0
		err = s.pigpioErrors(int(s.res))
		test.That(t, err, test.ShouldBeNil)
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		s.res = 1
		err = s.pigpioErrors(int(s.res))
		test.That(t, err, test.ShouldBeNil)
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		err = s.pigpioErrors(-4)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed")
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		err = s.Move(ctx, 8, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = s.Stop(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, err := s.Position(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})
}
