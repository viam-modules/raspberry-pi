package rpiutils

import (
	"context"
	"os/exec"

	"go.viam.com/rdk/logging"
)

// PerformReboot attempts to reboot the system using multiple fallback methods.
// It tries systemctl first, then sudo shutdown, and finally logs a warning if both fail.
func PerformReboot(logger logging.Logger) {
	// The reboot must not be cancelled when the calling request ends, so use a background context.
	if err := exec.CommandContext(context.Background(), "systemctl", "reboot").Run(); err != nil {
		logger.Debugf("systemctl reboot failed: %v", err)

		// TODO: Do you need sudo here?
		if err := exec.CommandContext(context.Background(), "sudo", "shutdown", "-r", "now").Run(); err != nil {
			logger.Debugf("sudo shutdown failed: %v", err)

			logger.Warnf("Automatic reboot failed. Please manually reboot the system for I2C changes to take effect: sudo reboot")
		}
	}
}
