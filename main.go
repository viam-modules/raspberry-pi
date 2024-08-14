package main

import (
	"context"
	"viamrpi/rpi"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"

	rpiservo "viamrpi/rpi-servo"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("raspberry-pi"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	module, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	if err = module.AddModelFromRegistry(ctx, board.API, rpi.Model); err != nil {
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
