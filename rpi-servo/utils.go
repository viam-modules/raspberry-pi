package rpiservo

/*
	Helper functions for the piservo.
*/

import "github.com/pkg/errors"

// Validate and set piPigpioServo fields based on the configuration.
func (s *piPigpioServo) validateAndSetConfiguration(conf *ServoConfig) error {
	if conf.Min >= 0 {
		s.min = uint32(conf.Min)
	}

	// Set to 180 if not set
	s.max = 180
	if conf.Max > 0 {
		s.max = uint32(conf.Max)
	}
	s.maxRotation = uint32(conf.MaxRotation)
	if s.maxRotation == 0 {
		s.maxRotation = uint32(servoDefaultMaxRotation)
	}
	if s.maxRotation < s.min {
		return errors.New("maxRotation is less than minimum")
	}
	if s.maxRotation < s.max {
		return errors.New("maxRotation is less than maximum")
	}

	s.pinname = conf.Pin

	return nil
}
