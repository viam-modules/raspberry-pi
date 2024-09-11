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

	// Kill any running instance of pigpiod.
	// This is important because there are cases where pigpiod
	// is running but is in a bad state. It's better to kill the existing
	// process and restart.
	killCmd := exec.CommandContext(ctx, "sudo", "killall", "pigpiod")
	if err := killCmd.Run(); err != nil {
		logger.Debug("No existing pigpiod instance running, proceeding to start pigpiod")
	} else {
		logger.Debug("Killed existing pigpiod instance")
	}

	// Start a fresh instance of pigpiod
	startCmd := exec.CommandContext(ctx, "sudo", "pigpiod")
	if _, err := startCmd.Output(); err != nil {
		logger.Info("failed to start pigpiod")
		return err
	}

	logger.Info("pigpio started successfully")
	return nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	stopCmd := exec.Command("killall", "pigpiod")

	return stopCmd.Run()
}
