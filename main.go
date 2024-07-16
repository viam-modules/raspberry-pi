package main

import (
	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("raspberry-pi"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	rpi, err := module.NewModuleFromArgs(ctx, logger)

	if err != nil {
		return err
	}

	// rpi.AddModelFromRegistry(ctx, board.API, Model)

	err = rpi.Start(ctx)
	defer rpi.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
