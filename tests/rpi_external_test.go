/* rpi_test runs same tests as rpi_test, but using exported board functions only */
package rpi_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
	"raspberry-pi/rpi"
	rpiservo "raspberry-pi/rpi-servo"
	rpiutils "raspberry-pi/utils"
)

func TestPiPigpio(t *testing.T) {
	piReg, ok := resource.LookupRegistration(board.API, rpi.Model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, piReg, test.ShouldNotBeNil)

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	cfg := rpi.Config{
		Pins: []rpiutils.PinConfig{
			{Name: "i1", Pin: "11", Type: "interrupt"}, // bcom 17
		},
	}

	piInt, err := piReg.Constructor(
		ctx,
		nil,
		resource.Config{
			Name:                "rpi",
			ConvertedAttributes: &cfg,
		},
		logger,
	)

	if os.Getuid() != 0 || err != nil && err.Error() == "not running on a pi" {
		t.Skip("not running as root on a pi")
		return
	}

	test.That(t, err, test.ShouldBeNil)
	p := piInt.(board.Board)

	defer func() {
		err := p.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	}()

	t.Run("external gpio and pwm", func(t *testing.T) {
		pin, err := p.GPIOPinByName("29")
		test.That(t, err, test.ShouldBeNil)

		// try to set high
		err = pin.Set(ctx, true, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err := pin.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		// try to set low
		err = pin.Set(ctx, false, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err = pin.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, false)

		// pwm 50%
		err = pin.SetPWM(ctx, 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		vF, err := pin.PWM(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.5, 0.01)

		// 4000 hz
		err = pin.SetPWMFreq(ctx, 4000, nil)
		test.That(t, err, test.ShouldBeNil)

		vI, err := pin.PWMFreq(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 4000)

		// 90%
		err = pin.SetPWM(ctx, 0.9, nil)
		test.That(t, err, test.ShouldBeNil)

		vF, err = pin.PWM(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.9, 0.01)

		// 8000hz
		err = pin.SetPWMFreq(ctx, 8000, nil)
		test.That(t, err, test.ShouldBeNil)

		vI, err = pin.PWMFreq(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 8000)
	})

	// interrupt is configured on pi board creation
	t.Run("external preconfigured basic interrupt test", func(t *testing.T) {
		// Test interrupt i1 on pin 11 (bcom 17)
		i1, err := p.DigitalInterruptByName("i1")
		test.That(t, err, test.ShouldBeNil)

		// set pin state to LOW for interrupt test
		pin, err := p.GPIOPinByName("11")
		test.That(t, err, test.ShouldBeNil)
		err = pin.Set(ctx, false, nil)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		before, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		// set pin state to HIGH to trigger interrupt
		err = pin.Set(ctx, true, nil)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	// digital interrupt creates by name (on valid pin)
	t.Run("external create new basic interrupt test", func(t *testing.T) {
		// Set and create interrupt on pin 13
		i2, err := p.DigitalInterruptByName("13")
		test.That(t, err, test.ShouldBeNil)

		// Set pin 13 (bcom 27) to LOW
		pin, err := p.GPIOPinByName("13")
		test.That(t, err, test.ShouldBeNil)
		err = pin.Set(ctx, false, nil)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		// interrupt not created, bad pin name
		_, err = p.DigitalInterruptByName("some")
		test.That(t, err, test.ShouldNotBeNil)

		before, err := i2.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		// Set pin 13 (bcom 27) to HIGH
		err = pin.Set(ctx, true, nil)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i2.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))

		_, err = p.DigitalInterruptByName("11")
		test.That(t, err, test.ShouldBeNil)
	})

	// test servo movement and digital interrupt
	// this function is within rpi in order to access piPigpio
	t.Run("external servo in/out", func(t *testing.T) {
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

		// Move to 90 deg and check position
		err = servo1.Move(ctx, 90, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 90)

		// should move to max position even though 190 is out of range
		err = servo1.Move(ctx, 190, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err = servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 180)

		time.Sleep(300 * time.Millisecond)

		// Next position (120 deg)
		err = servo1.Move(ctx, 120, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err = servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 120)
	})
}
