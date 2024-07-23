package rpiservo

import (
	"context"
	"testing"

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

func TestServoFunctions(t *testing.T) {
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

	t.Run(("check Move IsMoving ande pigpio errors"), func(t *testing.T) {
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
