package rpi

import (
	"os/exec"
)

// startPigpiod starts the pigpiod daemon.
func startPigpiod() error {
	startCmd := exec.Command("pigpiod")
	return startCmd.Run()
}

// stopPigpiod stops the pigpiod daemon.
func stopPigpiod() error {
	stopCmd := exec.Command("killall", "pigpiod")

	return stopCmd.Run()
}
