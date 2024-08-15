package rpi

import (
	"os/exec"
)

// startPigpiod starts the pigpiod daemon if it is not already running.
// Uses pgrep to check if pigpiod process exists, if not, start it.
// Assumes viam-server is running as root.
func startPigpiod() (bool, error) {
	checkCmd := exec.Command("pgrep", "pigpiod")
	output, err := checkCmd.Output()

	if err != nil || len(output) == 0 {
		// pigpiod is not running, start it
		startCmd := exec.Command("pigpiod")

		return false, startCmd.Run()
	}
	return true, nil
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	stopCmd := exec.Command("killall", "pigpiod")

	return stopCmd.Run()
}
