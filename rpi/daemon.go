package rpi

import (
	"context"
	"os/exec"
	"time"

	"go.viam.com/rdk/logging"
)

// This is a constant timeout for starting and stopping the pigpio daemon
var CTX_TIMEOUT = 10 * time.Second

// startPigpiod tries to start the pigpiod daemon.
// It returns an error if the daemon fails to start.
func startPigpiod(logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), CTX_TIMEOUT)
	defer cancel()

	// check if pigpio is active
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "pigpiod")
	if err := statusCmd.Run(); err != nil {
		startCmd := exec.CommandContext(ctx, "systemctl", "restart", "pigpiod")
		if err := startCmd.Run(); err != nil {
			logger.Debug("failed to restart pigpiod")
			return err
		}
	}
	logger.Info("pigpiod is already running")
	return nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	ctx, cancel := context.WithTimeout(context.Background(), CTX_TIMEOUT)
	defer cancel()

	stopCmd := exec.CommandContext(ctx, "systemctl", "stop", "pigpiod")
	return stopCmd.Run()
}
