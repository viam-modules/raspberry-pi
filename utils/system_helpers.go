package rpiutils

import (
	"os/exec"

	"go.viam.com/rdk/logging"
)

// PerformReboot attempts to reboot the system using multiple fallback methods.
// It tries systemctl first, then sudo shutdown, and finally logs a warning if both fail.
func PerformReboot(logger logging.Logger) {
	if err := exec.Command("systemctl", "reboot").Run(); err != nil {
		logger.Debugf("systemctl reboot failed: %v", err)

		// TODO: Do you need sudo here?
		if err := exec.Command("sudo", "shutdown", "-r", "now").Run(); err != nil {
			logger.Debugf("sudo shutdown failed: %v", err)

			logger.Warnf("Automatic reboot failed. Please manually reboot the system for I2C changes to take effect: sudo reboot")
		}
	}
}
