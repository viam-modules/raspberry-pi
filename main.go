// package main is a module with raspberry pi board component.
package main

import (
	"context"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"
	"raspberry-pi/pi5"
	"raspberry-pi/rpi"
	rpiservo "raspberry-pi/rpi-servo"
)

// func init() {
// 	if isPi5 {

// 		pi5.RegisterPINCTRL()
// 	} else {
// 		// init registers a pi board based on pigpio.
// 		rpi.RegisterPIGPIO()
// 	}
// }

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("raspberry-pi"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	module, err := module.NewModuleFromArgs(ctx)
	if err != nil {
		return err
	}

	if err = module.AddModelFromRegistry(ctx, board.API, pi5.Model); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi4); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi3); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi2); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi1); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi0_2); err != nil {
		return err
	}
	if err = module.AddModelFromRegistry(ctx, board.API, rpi.ModelPi0); err != nil {
		return err
	}

	if err = module.AddModelFromRegistry(ctx, servo.API, rpiservo.Model); err != nil {
		return err
	}

	err = module.Start(ctx)
	defer module.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
