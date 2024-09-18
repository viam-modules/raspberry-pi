package rpi

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"go.viam.com/rdk/logging"
)

// This is a constant timeout for starting and stopping the pigpio daemon.
const startStopTimeout = 10 * time.Second
const checkInterval = 50 * time.Millisecond

// startPigpiod tries to start the pigpiod daemon.
// It returns an error if the daemon fails to start.
func startPigpiod(ctx context.Context, logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, startStopTimeout)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// check if pigpio is active
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "pigpiod")
	if err := statusCmd.Run(); err != nil {
		startCmd := exec.CommandContext(ctx, "systemctl", "restart", "pigpiod")
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart pigpiod: %w", err)
		}

		// This loop waits 15ms for pigpiod to be active after restart.
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout reached: pigpiod did not become active")

			case <-ticker.C:
				if err := statusCmd.Run(); err == nil {
					logger.Info("pigpiod is runing after restart")
					return nil
				}
			}
		}
	}
	logger.Info("pigpiod is already running")
	return nil
}
