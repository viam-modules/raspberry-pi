// Package rpiutils contains utilities shared by the Raspberry Pi board implementations,
// including board and pin configuration, Broadcom pin mapping, digital interrupts,
// pigpio error conversion, and helpers for editing boot config files and rebooting.
package rpiutils

import (
	"fmt"

	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/resource"
)

// RaspiFamily is the model family for the Raspberry Pi module.
var RaspiFamily = resource.NewModelFamily("viam", "raspberry-pi")

// BoardSettings contains board-level configuration options.
type BoardSettings struct {
	I2Cenable    bool  `json:"enable_i2c,omitempty"`
	SPIenable    bool  `json:"enable_spi,omitempty"`
	BTenableuart *bool `json:"bluetooth_enable_uart,omitempty"`
	BTdtoverlay  *bool `json:"bluetooth_dtoverlay_miniuart,omitempty"`
	BTkbaudrate  *int  `json:"bluetooth_baud_rate,omitempty"`
}

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	Pins          []PinConfig                         `json:"pins,omitempty"`
	BoardSettings BoardSettings                       `json:"board_settings"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	for idx, c := range conf.AnalogReaders {
		err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx))
		if err != nil {
			return nil, nil, err
		}
	}

	for _, c := range conf.Pins {
		err := c.Validate(path)
		if err != nil {
			return nil, nil, err
		}
	}
	return nil, nil, nil
}
