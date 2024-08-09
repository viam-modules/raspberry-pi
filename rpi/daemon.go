package rpi

import (
	"os/exec"

	"go.viam.com/rdk/logging"
)

func startPigpiod(logger logging.Logger) error {
	checkCmd := exec.Command("pgrep", "pigpiod")
	output, err := checkCmd.Output()

	if err != nil || len(output) == 0 {
		// pigpiod is not running, start it
		startCmd := exec.Command("sudo", "pigpiod")
		err := startCmd.Run()

		return err
	}

	logger.Info("pigpiod is already running, skipping start")
	return nil
}

func stopPigpiod() error {
	cmd := exec.Command("killall", "pigpiod")
	err := cmd.Run()

	return err
}
