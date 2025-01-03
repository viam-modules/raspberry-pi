// package main is a module with raspberry pi board component.
package main

import (
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"raspberry-pi/pi5"
	"raspberry-pi/rpi"
	rpiservo "raspberry-pi/rpi-servo"
)

func main() {
	module.ModularMain(
		resource.APIModel{board.API, pi5.Model},
		resource.APIModel{board.API, rpi.ModelPi},
		resource.APIModel{board.API, rpi.ModelPi4},
		resource.APIModel{board.API, rpi.ModelPi3},
		resource.APIModel{board.API, rpi.ModelPi2},
		resource.APIModel{board.API, rpi.ModelPi1},
		resource.APIModel{board.API, rpi.ModelPi0_2},
		resource.APIModel{board.API, rpi.ModelPi0},
		resource.APIModel{servo.API, rpiservo.Model})
}
