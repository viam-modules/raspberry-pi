package rpi

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"go.viam.com/rdk/logging"
)

// This is a constant timeout for starting and stopping the pigpio daemon
const startStopTimeout = 10 * time.Second

// startPigpiod tries to start the pigpiod daemon.
// It returns an error if the daemon fails to start.
func startPigpiod(ctx context.Context, logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, startStopTimeout)
	defer cancel()

	// check if pigpio is active
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "pigpiod")
	if err := statusCmd.Run(); err != nil {
		startCmd := exec.CommandContext(ctx, "systemctl", "restart", "pigpiod")
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart pigpiod: %w", err)
		}
	}
	logger.Info("pigpiod is already running")
	return nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, startStopTimeout)
	defer cancel()

	stopCmd := exec.CommandContext(ctx, "systemctl", "stop", "pigpiod")
	return stopCmd.Run()
}
