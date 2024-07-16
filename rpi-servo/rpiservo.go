// Package rpiservo implements pi servo
package rpiservo

/*
	This driver contains various functionalities of a servo motor used in
	conjunction with a Raspberry Pi. The servo connects via a GPIO pin and
	uses the pi module's pigpio daemon library to control the servo motor.
	The servo pin will override the default pin configuration of of the pi
	module, including PWM frequency and width.
*/

import "go.viam.com/rdk/resource"

var Model = resource.NewModel("viam", "raspberry-pi", "rpi-servo")
