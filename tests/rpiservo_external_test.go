package rpi_test

import (
	"context"
	"testing"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
	rpiservo "raspberry-pi/rpi-servo"
)

func TestPiServo(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("servo initialize with pin error", func(t *testing.T) {
		servoReg, ok := resource.LookupRegistration(servo.API, rpiservo.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		_, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &rpiservo.ServoConfig{Pin: ""},
			},
			logger,
		)
		test.That(t, err.Error(), test.ShouldContainSubstring, "need pin for pi servo")
	})

	t.Run("check new servo defaults", func(t *testing.T) {
		ctx := context.Background()
		servoReg, ok := resource.LookupRegistration(servo.API, rpiservo.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &rpiservo.ServoConfig{Pin: "22"},
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
		servoReg, ok := resource.LookupRegistration(servo.API, rpiservo.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)

		initPos := 33.0
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &rpiservo.ServoConfig{Pin: "22", StartPos: &initPos},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		servo1 := servoInt.(servo.Servo)
		pos1, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos1, test.ShouldEqual, 33)
	})
}
