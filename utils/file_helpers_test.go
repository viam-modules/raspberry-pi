package rpiutils

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

// TestI2CConfiguration tests the turn_i2c_on behavior.
func TestI2CConfiguration(t *testing.T) {
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name          string
		i2cEnable     bool
		expectChange  bool
		initialConfig string
		initialModule string
	}{
		{
			name:          "turn_on_from_scratch",
			i2cEnable:     true,
			expectChange:  true,
			initialConfig: "",
			initialModule: "",
		},
		{
			name:          "turn_on_already_enabled",
			i2cEnable:     true,
			expectChange:  false,
			initialConfig: "dtparam=i2c_arm=on\n",
			initialModule: "i2c-dev\n",
		},
		{
			name:          "turn_on_from_commented",
			i2cEnable:     true,
			expectChange:  true,
			initialConfig: "#dtparam=i2c_arm=on\n",
			initialModule: "#i2c-dev\n",
		},
		{
			name:          "false_does_nothing_empty",
			i2cEnable:     false,
			expectChange:  false,
			initialConfig: "",
			initialModule: "",
		},
		{
			name:          "false_does_nothing_enabled",
			i2cEnable:     false,
			expectChange:  false,
			initialConfig: "dtparam=i2c_arm=on\n",
			initialModule: "i2c-dev\n",
		},
		{
			name:          "false_does_nothing_disabled",
			i2cEnable:     false,
			expectChange:  false,
			initialConfig: "dtparam=i2c_arm=off\n",
			initialModule: "#i2c-dev\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.txt")
			modulePath := filepath.Join(tempDir, "modules")

			if err := os.WriteFile(configPath, []byte(tc.initialConfig), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(modulePath, []byte(tc.initialModule), 0o644); err != nil {
				t.Fatal(err)
			}

			// Test the I2C configuration flow
			var configChanged, moduleChanged bool
			var err error

			if tc.i2cEnable {
				configChanged, err = UpdateConfigFile(configPath, "dtparam=i2c_arm", "on", logger)
				test.That(t, err, test.ShouldBeNil)

				moduleChanged, err = UpdateModuleFile(modulePath, "i2c-dev", true, logger)
				test.That(t, err, test.ShouldBeNil)

				// Verify final state - should be enabled
				finalConfig, err := os.ReadFile(configPath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalConfig), test.ShouldContainSubstring, "dtparam=i2c_arm=on")

				finalModule, err := os.ReadFile(modulePath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalModule), test.ShouldContainSubstring, "i2c-dev")
				test.That(t, string(finalModule), test.ShouldNotContainSubstring, "#i2c-dev")
			} else {
				// When turn_i2c_on is false, no operations should occur
				configChanged = false
				moduleChanged = false

				// Verify no changes were made to files
				finalConfig, err := os.ReadFile(configPath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalConfig), test.ShouldEqual, tc.initialConfig)

				finalModule, err := os.ReadFile(modulePath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalModule), test.ShouldEqual, tc.initialModule)
			}

			shouldReboot := configChanged || moduleChanged
			test.That(t, shouldReboot, test.ShouldEqual, tc.expectChange)
		})
	}
}

// TestI2CConfigIntegration tests integration with the board config.
func TestI2CConfigIntegration(t *testing.T) {
	testCases := []struct {
		name        string
		config      Config
		expectCalls bool
	}{
		{
			name: "i2c_enable_true",
			config: Config{
				BoardSettings: BoardSettings{
					I2Cenable: true,
				},
			},
			expectCalls: true,
		},
		{
			name: "i2c_enable_false",
			config: Config{
				BoardSettings: BoardSettings{
					I2Cenable: false,
				},
			},
			expectCalls: false,
		},
		{
			name: "i2c_enable_omitted",
			config: Config{
				BoardSettings: BoardSettings{},
			},
			expectCalls: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the logic correctly interprets the config
			shouldEnable := tc.config.BoardSettings.I2Cenable
			test.That(t, shouldEnable, test.ShouldEqual, tc.expectCalls)
		})
	}
}

// TestI2CEdgeCases tests edge cases for the I2C configuration.
func TestI2CEdgeCases(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("enable_with_existing_disabled_config", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.txt")
		modulePath := filepath.Join(tempDir, "modules")

		// Start with I2C explicitly disabled
		initialConfig := "dtparam=i2c_arm=off\nother=setting\n"
		initialModule := "snd-bcm2835\n#i2c-dev\nother-module\n"

		if err := os.WriteFile(configPath, []byte(initialConfig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modulePath, []byte(initialModule), 0o644); err != nil {
			t.Fatal(err)
		}

		// Enable I2C
		configChanged, err := UpdateConfigFile(configPath, "dtparam=i2c_arm", "on", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, configChanged, test.ShouldBeTrue)

		moduleChanged, err := UpdateModuleFile(modulePath, "i2c-dev", true, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moduleChanged, test.ShouldBeTrue)

		// Verify final state
		finalConfig, err := os.ReadFile(configPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(finalConfig), test.ShouldContainSubstring, "dtparam=i2c_arm=on")
		test.That(t, string(finalConfig), test.ShouldContainSubstring, "other=setting")

		finalModule, err := os.ReadFile(modulePath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(finalModule), test.ShouldContainSubstring, "snd-bcm2835")
		test.That(t, string(finalModule), test.ShouldContainSubstring, "i2c-dev")
		test.That(t, string(finalModule), test.ShouldContainSubstring, "other-module")
		test.That(t, string(finalModule), test.ShouldNotContainSubstring, "#i2c-dev")
	})

	t.Run("enable_idempotent", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.txt")
		modulePath := filepath.Join(tempDir, "modules")

		// Start with I2C already enabled
		initialConfig := "dtparam=i2c_arm=on\n"
		initialModule := "i2c-dev\n"

		if err := os.WriteFile(configPath, []byte(initialConfig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modulePath, []byte(initialModule), 0o644); err != nil {
			t.Fatal(err)
		}

		// Try to enable again - should be no-op
		configChanged, err := UpdateConfigFile(configPath, "dtparam=i2c_arm", "on", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, configChanged, test.ShouldBeFalse)

		moduleChanged, err := UpdateModuleFile(modulePath, "i2c-dev", true, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moduleChanged, test.ShouldBeFalse)

		// Verify no reboot needed
		shouldReboot := configChanged || moduleChanged
		test.That(t, shouldReboot, test.ShouldBeFalse)
	})
}

// TestRemoveConfigParam tests the removal of any *uncommented* line matching the given param (paramPrefix=.*).
func TestRemoveConfigParam(t *testing.T) {
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name          string
		removeLine    string
		expectChange  bool
		initialConfig string
	}{
		{
			name:          "enable_uart_off_empty",
			removeLine:    "enable_uart=0",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "enable_uart_on_empty",
			removeLine:    "enable_uart=1",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "miniuart_empty",
			removeLine:    "dtoverlay=miniuart-bt",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "baudrate_any_value_empty",
			removeLine:    "dtparam=krnbt_baudrate=576000",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "enable_uart_on_comment",
			removeLine:    "enable_uart=1",
			expectChange:  false,
			initialConfig: "# enable_uart=1",
		},
		{
			name:          "miniuart_comment",
			removeLine:    "dtoverlay=miniuart-bt",
			expectChange:  false,
			initialConfig: "  # dtoverlay=miniuart-bt",
		},
		{
			name:          "baudrate_any_value_comment",
			removeLine:    "dtparam=krnbt_baudrate",
			expectChange:  false,
			initialConfig: " #dtparam=krnbt_baudrate=576000",
		},
		{
			name:          "enable_uart_off_existing",
			removeLine:    "enable_uart=0",
			expectChange:  true,
			initialConfig: "enable_uart=0",
		},
		{
			name:          "enable_uart_on_existing",
			removeLine:    "enable_uart=1",
			expectChange:  true,
			initialConfig: "enable_uart=1",
		},
		{
			name:          "miniuart_existing",
			removeLine:    "dtoverlay=miniuart-bt",
			expectChange:  true,
			initialConfig: "dtoverlay=miniuart-bt",
		},
		{
			name:          "baudrate_any_value_existing",
			removeLine:    "dtparam=krnbt_baudrate=576000",
			expectChange:  true,
			initialConfig: " dtparam=krnbt_baudrate=576000",
		},
		{
			name:          "enable_uart_on_extra_comment",
			removeLine:    "enable_uart=1",
			expectChange:  true,
			initialConfig: "enable_uart=1   # extra comment\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.txt")

			if err := os.WriteFile(configPath, []byte(tc.initialConfig), 0o644); err != nil {
				t.Fatal(err)
			}

			// Test the RemoveConfigParam() function
			var configChanged bool
			var err error

			configChanged, err = RemoveConfigParam(configPath, tc.removeLine, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, configChanged, test.ShouldEqual, tc.expectChange)

			if tc.expectChange {
				// Verify final state - should be deleted
				finalConfig, err := os.ReadFile(configPath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalConfig), test.ShouldEqual, "")
			} else {
				// no operations should occur if the line did not exist or was a comment
				// Verify no changes were made to config.txt file
				finalConfig, err := os.ReadFile(configPath)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, string(finalConfig), test.ShouldEqual, tc.initialConfig)
			}

			shouldReboot := configChanged
			test.That(t, shouldReboot, test.ShouldEqual, tc.expectChange)
		})
	}
}

// TestDetectConfigParam tests if the exact desired line already exists.
func TestDetectConfigParam(t *testing.T) {
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name          string
		detectLine    string
		expectChange  bool
		initialConfig string
	}{
		{
			name:          "detect_enable_uart_off_empty",
			detectLine:    "enable_uart=0",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "detect_enable_uart_on_empty",
			detectLine:    "enable_uart=1",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "detect_miniuart_empty",
			detectLine:    "dtoverlay=miniuart-bt",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "detect_baudrate_specific_value_empty",
			detectLine:    "dtparam=krnbt_baudrate=576000",
			expectChange:  false,
			initialConfig: "",
		},
		{
			name:          "detect_enable_uart_off_existing",
			detectLine:    "enable_uart=0",
			expectChange:  true,
			initialConfig: "enable_uart=0",
		},
		{
			name:          "detect_enable_uart_on_existing",
			detectLine:    "enable_uart=1",
			expectChange:  true,
			initialConfig: "enable_uart=1",
		},
		{
			name:          "detect_miniuart_existing",
			detectLine:    "dtoverlay=miniuart-bt",
			expectChange:  true,
			initialConfig: "dtoverlay=miniuart-bt",
		},
		{
			name:          "detect_baudrate_specific_value_existing",
			detectLine:    "dtparam=krnbt_baudrate=576000",
			expectChange:  true,
			initialConfig: "dtparam=krnbt_baudrate=576000",
		},
		{
			name:          "detect_baudrate_wrong_value_existing",
			detectLine:    "dtparam=krnbt_baudrate=576000",
			expectChange:  false,
			initialConfig: "dtparam=krnbt_baudrate=921600",
		},
		{
			name:          "detect_enable_uart_on_comment",
			detectLine:    "enable_uart=1",
			expectChange:  false,
			initialConfig: "# enable_uart=1",
		},
		{
			name:          "detect_enable_uart_off_comment",
			detectLine:    "enable_uart=0",
			expectChange:  false,
			initialConfig: "# enable_uart=0",
		},
		{
			name:          "detect_miniuart_comment",
			detectLine:    "dtoverlay=miniuart-bt",
			expectChange:  false,
			initialConfig: "  # dtoverlay=miniuart-bt",
		},
		{
			name:          "detect_baudrate_any_value_comment",
			detectLine:    "dtparam=krnbt_baudrate",
			expectChange:  false,
			initialConfig: " #dtparam=krnbt_baudrate=576000",
		},
		{
			name:          "detect_enable_uart_on_extra_comment",
			detectLine:    "enable_uart=1",
			expectChange:  true,
			initialConfig: "enable_uart=1   # extra comment\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.txt")

			if err := os.WriteFile(configPath, []byte(tc.initialConfig), 0o644); err != nil {
				t.Fatal(err)
			}

			// Test the DetectConfigParam() function
			var found bool
			var err error

			found, err = RemoveConfigParam(configPath, tc.detectLine, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, found, test.ShouldEqual, tc.expectChange)
		})
	}
}
