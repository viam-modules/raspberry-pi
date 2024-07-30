// Package rpiservo contains servo config to ensure it is valid with a pin and board name.
package rpiservo

import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/resource"
)

// ServoConfig is the config for a pi servo.
type ServoConfig struct {
	BoardName string `json:"board"`
	Pin       string `json:"pin"`

	Min         int      `json:"min,omitempty"`                    // specifies a user inputted minimum position limitation
	Max         int      `json:"max,omitempty"`                    // specifies a user inputted maximum position limitation
	StartPos    *float64 `json:"starting_position_degs,omitempty"` // specifies a starting position. Defaults to 90
	HoldPos     *bool    `json:"hold_position,omitempty"`          // defaults True. False holds for 500 ms then disables servo
	MaxRotation int      `json:"max_rotation_deg,omitempty"`       // specifies a hardware position limitation. Defaults to 180
}

// Validate ensures all parts of the config are valid.
func (config *ServoConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.Pin == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("need pin for pi servo"))
	}
	if config.BoardName == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("need the name of the board"))
	}
	deps = append(deps, config.BoardName)
	return deps, nil
}
