package rpi

/*
	This file implements analog reader functionality for the Raspberry Pi.
*/

import (
	"strconv"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/components/board/pinwrappers"
)

// Helper functions to configure analog readers and interrupts.
func (pi *piPigpio) reconfigureAnalogReaders(cfg *Config) error {
	// No need to reconfigure the old analog readers; just throw them out and make new ones.
	pi.analogReaders = map[string]*pinwrappers.AnalogSmoother{}
	for _, ac := range cfg.AnalogReaders {
		channel, err := strconv.Atoi(ac.Channel)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", ac.Channel)
		}

		chipSelect := ac.ChipSelect

		// Use genericlinux implementation for SPI bus.
		switch chipSelect {
		case "24", "ce0", "io8", "0":
			// HW pin 24 maps to chip select 0
			chipSelect = "0"
		case "26", "ce1", "io7", "1":
			// HW pin 26 maps to chip select 1
			chipSelect = "1"
		default:
			return errors.Errorf("bad chip select (%s), choose chip select 0 (pin 24) or 1 (pin 26)", chipSelect)
		}

		bus := buses.NewSpiBus(ac.SPIBus)

		ar := &mcp3008helper.MCP3008AnalogReader{
			Channel: channel,
			Bus:     bus,
			Chip:    chipSelect,
		}

		pi.analogReaders[ac.Name] = pinwrappers.SmoothAnalogReader(ar, board.AnalogReaderConfig{
			AverageOverMillis: ac.AverageOverMillis, SamplesPerSecond: ac.SamplesPerSecond,
		}, pi.logger)
	}
	return nil
}

// AnalogNames returns the names of all known analog pins.
func (pi *piPigpio) AnalogNames() []string {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	names := []string{}
	for k := range pi.analogReaders {
		names = append(names, k)
	}
	return names
}

// AnalogByName returns an analog pin by name.
func (pi *piPigpio) AnalogByName(name string) (board.Analog, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	a, ok := pi.analogReaders[name]
	if !ok {
		return nil, errors.Errorf("can't find Analog pin (%s)", name)
	}
	return a, nil
}
