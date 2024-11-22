// Package rpiutils contains implementations for digital_interrupts here.
package rpiutils

import (
	"fmt"

	"go.viam.com/rdk/components/board/mcp3008helper"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	Pins          []PinConfig                         `json:"pins,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, c := range conf.AnalogReaders {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}

	for _, c := range conf.Pins {
		if err := c.Validate(path); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
