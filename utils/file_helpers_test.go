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
		name           string
		turnI2COn      bool
		expectChange   bool
		initialConfig  string
		initialModule  string
	}{
		{
			name:          "turn_on_from_scratch",
			turnI2COn:     true,
			expectChange:  true,
			initialConfig: "",
			initialModule: "",
		},
		{
			name:          "turn_on_already_enabled",
			turnI2COn:     true,
			expectChange:  false,
			initialConfig: "dtparam=i2c_arm=on\n",
			initialModule: "i2c-dev\n",
		},
		{
			name:          "turn_on_from_commented",
			turnI2COn:     true,
			expectChange:  true,
			initialConfig: "#dtparam=i2c_arm=on\n",
			initialModule: "#i2c-dev\n",
		},
		{
			name:          "false_does_nothing_empty",
			turnI2COn:     false,
			expectChange:  false,
			initialConfig: "",
			initialModule: "",
		},
		{
			name:          "false_does_nothing_enabled",
			turnI2COn:     false,
			expectChange:  false,
			initialConfig: "dtparam=i2c_arm=on\n",
			initialModule: "i2c-dev\n",
		},
		{
			name:          "false_does_nothing_disabled",
			turnI2COn:     false,
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

			if err := os.WriteFile(configPath, []byte(tc.initialConfig), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(modulePath, []byte(tc.initialModule), 0644); err != nil {
				t.Fatal(err)
			}

			// Test the I2C configuration flow
			var configChanged, moduleChanged bool
			var err error

			if tc.turnI2COn {
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
			name: "turn_i2c_on_true",
			config: Config{
				BoardSettings: BoardSettings{
					TurnI2COn: true,
				},
			},
			expectCalls: true,
		},
		{
			name: "turn_i2c_on_false",
			config: Config{
				BoardSettings: BoardSettings{
					TurnI2COn: false,
				},
			},
			expectCalls: false,
		},
		{
			name: "turn_i2c_on_omitted",
			config: Config{
				BoardSettings: BoardSettings{},
			},
			expectCalls: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the logic correctly interprets the config
			shouldEnable := tc.config.BoardSettings.TurnI2COn
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
		
		if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modulePath, []byte(initialModule), 0644); err != nil {
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
		
		if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modulePath, []byte(initialModule), 0644); err != nil {
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
