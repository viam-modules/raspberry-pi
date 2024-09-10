package rpi

import (
	"os/exec"

	"go.viam.com/rdk/logging"
)

// startPigpiod tries to start the pigpiod daemon.
// It returns an error if the daemon fails to start.
func startPigpiod(logger logging.Logger) error {
	// Check if pigpiod is already running
	checkCmd := exec.Command("pgrep", "pigpiod")
	if output, err := checkCmd.Output(); err == nil && len(output) > 0 {
		// pigpiod is already running, no need to start it
		logger.Info("pigpio is already running")
		return nil
	}

	// pigpiod is not running, attempt to start it
	startCmd := exec.Command("sudo", "pigpiod") // Using sudo to ensure necessary privileges

	if err := startCmd.Run(); err != nil {
		return err
	}

	// Check again if pigpiod started successfully
	checkCmd = exec.Command("pgrep", "pigpiod")
	if output, err := checkCmd.Output(); err != nil || len(output) == 0 {
		return err
	}

	return nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	stopCmd := exec.Command("killall", "pigpiod")

	return stopCmd.Run()
}
