package rpi

import (
	"context"
	"os"
	"testing"
	"time"

	rpiservo "raspberry-pi/rpi-servo"
	rpiutils "raspberry-pi/utils"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	cfg := rpiutils.Config{
		Pins: []rpiutils.PinConfig{
			{Name: "i1", Pin: "11", Type: "interrupt"}, // bcom 17
			{Name: "servo-i", Pin: "22", Type: "interrupt"},
			{Name: "blue", Pin: "33", Type: "gpio"},
		},
	}
	resourceConfig := resource.Config{
		Name:                "foo",
		ConvertedAttributes: &cfg,
	}

	pp, err := newPigpio(ctx, nil, resourceConfig, logger)
	if os.Getuid() != 0 || err != nil && err.Error() == "not running on a pi" {
		t.Skip("not running as root on a pi")
		return
	}
	test.That(t, err, test.ShouldBeNil)

	p := pp.(*piPigpio)

	defer func() {
		err := p.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	}()

	t.Run("test interrupts on reconfigure", func(t *testing.T) {
		expectedNumInterrupts := 2
		// before adding an interrupt using DigitalInterruptByName, confirm
		// pi.interrupts length matches the number of interrupts in the config
		test.That(t, len(p.interrupts), test.ShouldEqual, expectedNumInterrupts)

		// add a digital interrupt using DigitalInterruptByName,
		// check that it has been added to the pi.interrupts list
		_, err := p.DigitalInterruptByName("10")
		expectedNumInterrupts = 3
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(p.interrupts), test.ShouldEqual, expectedNumInterrupts)

		// test that the number of interrupts is the same after reconfigure
		err = p.Reconfigure(ctx, nil, resourceConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(p.interrupts), test.ShouldEqual, expectedNumInterrupts)
	})

	t.Run("gpio and pwm", func(t *testing.T) {
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

	t.Run("gpio pins respect names and can use hardware name", func(t *testing.T) {
		// blue, pin33, and io13 are all the same pin
		pinBlue, err := p.GPIOPinByName("blue")
		test.That(t, err, test.ShouldBeNil)
		pin33, err := p.GPIOPinByName("33")
		test.That(t, err, test.ShouldBeNil)
		pinIO13, err := p.GPIOPinByName("io13")
		test.That(t, err, test.ShouldBeNil)
		// pin 35 is a different pin
		diffPin, err := p.GPIOPinByName("35")
		test.That(t, err, test.ShouldBeNil)

		// try to set high
		err = pinBlue.Set(ctx, true, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err := pinBlue.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		v, err = pin33.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		v, err = pinIO13.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		v, err = diffPin.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, false)
	})

	// interrupt is configured on pi board creation
	t.Run("preconfigured basic interrupt test", func(t *testing.T) {
		// Test interrupt i1 on pin 11 (bcom 17)
		i1, err := p.DigitalInterruptByName("i1")
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(17, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		before, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(17, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	// digital interrupt creates by name (on valid pin)
	t.Run("create new basic interrupt test", func(t *testing.T) {
		// Set and create interrupt on pin 13
		i2, err := p.DigitalInterruptByName("13")
		test.That(t, err, test.ShouldBeNil)
		// Set pin 13 (bcom 27) to LOW
		err = p.SetGPIOBcom(27, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		// interrupt not created, bad pin name
		_, err = p.DigitalInterruptByName("some")
		test.That(t, err, test.ShouldNotBeNil)

		before, err := i2.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		// Set pin 13 (bcom 27) to HIGH
		err = p.SetGPIOBcom(27, true)
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
	t.Run("servo in/out", func(t *testing.T) {
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

		servoI, err := p.DigitalInterruptByName("servo-i")
		test.That(t, err, test.ShouldBeNil)
		val, err := servoI.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldAlmostEqual, int64(2500), 100) // this is a tad noisy

		// Next position (120 deg)
		err = servo1.Move(ctx, 120, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err = servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 120)

		time.Sleep(300 * time.Millisecond)
		val, err = servoI.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldAlmostEqual, int64(1833), 50) // this is a tad noisy
	})
}
