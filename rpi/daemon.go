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
	startCmd := exec.Command("sudo", "pigpiod")

	if err := startCmd.Run(); err != nil {
		return err
	}

	// there are cases where we may execute sudo pigpiod but
	// it failed to start. The follwoing check is there to ensure that
	// sudo pigpio was executed successfully.
	checkCmd = exec.Command("pgrep", "pigpiod")
	if output, err := checkCmd.Output(); err != nil || len(output) == 0 {
		logger.Warn("could not start pigpiod")
		return err
	}

	logger.Info("pigpio started")
	return nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	stopCmd := exec.Command("killall", "pigpiod")

	return stopCmd.Run()
}
