package rpi

import (
	"context"
	"os/exec"
	"time"

	"go.viam.com/rdk/logging"
)

// startPigpiod tries to start the pigpiod daemon.
// It returns an error if the daemon fails to start.
func startPigpiod(logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// check if pigpio is active
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "pigpiod")
	err := statusCmd.Run()
	if err != nil {
		startCmd := exec.CommandContext(ctx, "systemctl", "restart", "pigpiod")
		if err := startCmd.Run(); err != nil {
			logger.Debug("failed to restart pigpiod")
			return err
		}
	}
	logger.Info("pigpiod is already running")
	return err
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopCmd := exec.CommandContext(ctx, "systemctl", "stop", "pigpiod")
	return stopCmd.Run()
}
