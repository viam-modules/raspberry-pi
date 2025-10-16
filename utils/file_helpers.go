package rpiutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.viam.com/rdk/logging"
)

// UpdateConfigFile atomically updates a configuration file parameter using regexp.
// - Replaces existing uncommented param lines with the desired value
// - Leaves commented lines intact
// - Appends only if the (uncommented) line exists
// - Preserves file permissions (uses os.Stat + os.WriteFile with original mode)
// - Atomic via temp file + rename
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
	hasActiveTarget := false
	targetLine := paramPrefix + desiredValue

	// Matches uncommented or commented param lines (skip commented lines below)
	re := regexp.MustCompile(fmt.Sprintf(`^\s*#?\s*%s.*$`, regexp.QuoteMeta(targetLine)))

	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}

		trimmed := strings.TrimSpace(line)

		// Do not modify commented lines
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Existing active param line
		if trimmed == targetLine {
			hasActiveTarget = true
			continue
		}

		// Replace an active param line to the target value
		lines[i] = targetLine
		configChanged = true
		hasActiveTarget = true // prevent duplicate append
	}

	// Append only if no active target line was found
	if !hasActiveTarget {
		lines = append(lines, targetLine)
		configChanged = true
	}

	if !configChanged {
		return false, nil
	}

	newContent := strings.Join(lines, "\n")
	tempFile := filePath + ".tmp"

	if err := os.WriteFile(tempFile, []byte(newContent), fileInfo.Mode()); err != nil {
		return false, fmt.Errorf("failed to write temp config file %s: %w", tempFile, err)
	}
	if err := os.Rename(tempFile, filePath); err != nil {
		_ = os.Remove(tempFile)
		return false, fmt.Errorf("failed to replace config file %s: %w", filePath, err)
	}

	logger.Debugf("Updated %s in %s", paramPrefix, filePath)
	return true, nil
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

// RemoveLineMatching removes every uncommented line that matches the given regular expression.
// Returns true if any line was removed. Preserves file permissions and writes atomically.
func RemoveLineMatching(filePath string, lineRegex *regexp.Regexp, logger logging.Logger) (bool, error) {
	filePath = filepath.Clean(filePath)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat config file %s: %w", filePath, err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	origLines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(origLines))
	removed := false

	for _, line := range origLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			filtered = append(filtered, line)
			continue // skip comments entirely
		}
		if lineRegex.MatchString(line) {
			removed = true
			continue
		}
		filtered = append(filtered, line)
	}

	if !removed {
		return false, nil
	}

	newContent := strings.Join(filtered, "\n")
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, []byte(newContent), fileInfo.Mode()); err != nil {
		return false, fmt.Errorf("failed to write temp config file %s: %w", tempFile, err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		_ = os.Remove(tempFile)
		return false, fmt.Errorf("failed to replace config file %s: %w", filePath, err)
	}

	logger.Debugf("Removed uncommented lines matching %q in %s", lineRegex.String(), filePath)
	return true, nil
}

// RemoveConfigParam removes any *uncommented* line defining the given param (paramPrefix=.*).
func RemoveConfigParam(filePath, paramPrefix string, logger logging.Logger) (bool, error) {
	re := regexp.MustCompile(fmt.Sprintf(`^\s*%s.*$`, regexp.QuoteMeta(paramPrefix)))
	return RemoveLineMatching(filePath, re, logger)
}
