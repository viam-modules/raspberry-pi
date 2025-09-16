package rpiutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/rdk/logging"
)

// UpdateConfigFile atomically updates a configuration file parameter.
// It handles multiple entries, commented lines, and preserves file permissions
func UpdateConfigFile(filePath, paramPrefix, desiredValue string, logger logging.Logger) (bool, error) {
	filePath = filepath.Clean(filePath)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat config file %s: %w", filePath, err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	lines := strings.Split(string(content), "\n")
	configChanged := false
	correctEntryExists := false
	targetLine := paramPrefix + "=" + desiredValue
	
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmedLine, paramPrefix+"=") {
			if trimmedLine == targetLine {
				correctEntryExists = true
			} else {
				lines[i] = targetLine
				configChanged = true
			}
		} else if strings.HasPrefix(trimmedLine, "#"+paramPrefix+"=") {
			lines[i] = targetLine
			configChanged = true
		}
	}

	if !correctEntryExists {
		lines = append(lines, targetLine)
		configChanged = true
	}

	if configChanged {
		newContent := strings.Join(lines, "\n")
		
		tempFile := filePath + ".tmp"
		if err := os.WriteFile(tempFile, []byte(newContent), fileInfo.Mode()); err != nil {
			return false, fmt.Errorf("failed to write temp config file %s: %w", tempFile, err)
		}

		if err := os.Rename(tempFile, filePath); err != nil {
			if removeErr := os.Remove(tempFile); removeErr != nil {
				logger.Warnf("Failed to clean up temp file %s: %v", tempFile, removeErr)
			}
			return false, fmt.Errorf("failed to replace config file %s: %w", filePath, err)
		}

		logger.Infof("Updated %s in %s", paramPrefix, filePath)
	}

	return configChanged, nil
}

// UpdateModuleFile atomically enables or disables a kernel module in /etc/modules.
// It handles commenting/uncommenting existing entries and preserves file permissions.
func UpdateModuleFile(filePath, moduleName string, enable bool, logger logging.Logger) (bool, error) {
	filePath = filepath.Clean(filePath)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat modules file %s: %w", filePath, err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read modules file %s: %w", filePath, err)
	}

	lines := strings.Split(string(content), "\n")
	moduleFound := false
	configChanged := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == moduleName {
			moduleFound = true
			if !enable {
				lines[i] = "#" + line
				configChanged = true
			}
		} else if trimmedLine == "#"+moduleName {
			if enable {
				lines[i] = moduleName
				configChanged = true
				moduleFound = true
			} else {
				moduleFound = true
			}
		}
	}

	if enable && !moduleFound {
		lines = append(lines, moduleName)
		configChanged = true
	}

	if configChanged {
		newContent := strings.Join(lines, "\n")
		
		tempFile := filePath + ".tmp"
		if err := os.WriteFile(tempFile, []byte(newContent), fileInfo.Mode()); err != nil {
			return false, fmt.Errorf("failed to write temp modules file %s: %w", tempFile, err)
		}

		if err := os.Rename(tempFile, filePath); err != nil {
			if removeErr := os.Remove(tempFile); removeErr != nil {
				logger.Warnf("Failed to clean up temp file %s: %v", tempFile, removeErr)
			}
			return false, fmt.Errorf("failed to replace modules file %s: %w", filePath, err)
		}

		action := "Added"
		if !enable {
			action = "Disabled"
		}
		logger.Infof("%s %s in %s", action, moduleName, filePath)
	}

	return configChanged, nil
}

// GetBootConfigPath returns the correct path for boot config file.
// Handles both /boot/config.txt (older) and /boot/firmware/config.txt (newer).
func GetBootConfigPath() string {
	if _, err := os.Stat("/boot/firmware/config.txt"); err == nil {
		return "/boot/firmware/config.txt"
	}
	return "/boot/config.txt"
}
