package rpi

/*
	This file implements digital interrupt functionality for the Raspberry Pi.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpiod_if2
import "C"

import (
	"context"
	"fmt"
	"math"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	rpiutils "raspberry-pi/utils"
)

type rpiInterrupt struct {
	interrupt            rpiutils.ReconfigurableDigitalInterrupt
	callbackID           C.uint // callback ID to close pi callback connection
	lastTicks            uint64
	debounceMicroSeconds uint64
}

// findInterruptByName finds an interrupt by its name, such as: "interrupt-1"
func findInterruptByName(
	name string,
	interrupts map[uint]*rpiInterrupt,
) (rpiutils.ReconfigurableDigitalInterrupt, bool) {
	for _, rpiInterrupt := range interrupts {
		if rpiInterrupt.interrupt.Name() == name {
			return rpiInterrupt.interrupt, true
		}
	}
	return nil, false
}

// reconfigureInterrupts reconfigures the digital interrupts based on the new configuration provided.
// It reuses existing interrupts when possible and creates new ones if necessary.
func (pi *piPigpio) reconfigureInterrupts(ctx context.Context, cfg *rpiutils.Config) error {
	// look at previous interrupt config, and see if we removed any
	for _, oldConfig := range pi.pinConfigs {
		if oldConfig.Type != rpiutils.PinInterrupt {
			continue
		}
		sameInterrupt := false
		for _, newConfig := range cfg.Pins {
			if newConfig.Type != rpiutils.PinInterrupt {
				continue
			}
			// check if we still have this interrupt
			if oldConfig.Name == newConfig.Name && oldConfig.Pin == newConfig.Pin {
				sameInterrupt = true
				break
			}
		}
		// if we still have the interrupt, don't modify it
		if sameInterrupt {
			continue
		}
		// we no longer want this interrupt, so we will remove it
		bcom, ok := rpiutils.BroadcomPinFromHardwareLabel(oldConfig.Pin)
		if !ok {
			return errors.Errorf("cannot find GPIO for unknown pin: %s", oldConfig.Name)
		}
		interrupt, ok := pi.interrupts[bcom]
		if ok {
			if result := C.teardownInterrupt(interrupt.callbackID); result != 0 {
				return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
			}
			delete(pi.interrupts, bcom)
		}
	}

	// Set new interrupts based on config
	for _, newConfig := range cfg.Pins {
		if newConfig.Type != rpiutils.PinInterrupt {
			continue
		}
		// check if pin is valid
		bcom, ok := rpiutils.BroadcomPinFromHardwareLabel(newConfig.Pin)
		if !ok {
			return errors.Errorf("no hw mapping for %s", newConfig.Pin)
		}

		// check if we are already managing pin
		interrupt, ok := pi.interrupts[bcom]
		if ok {
			pi.logger.Infof("interrupt %v is already a tracked interrupt", interrupt.interrupt.Name())
			continue
		}

		// create new interrupt
		_, err := pi.createNewInterrupt(newConfig, bcom)
		if err != nil {
			return err
		}
	}

	return nil
}

// createNewInterrupt creates a new digital interrupt and sets it up with the specified configuration.
func (pi *piPigpio) createNewInterrupt(newConfig rpiutils.PinConfig, bcom uint) (rpiutils.ReconfigurableDigitalInterrupt, error) {
	d, err := rpiutils.CreateDigitalInterrupt(
		rpiutils.PinConfig{
			Name: newConfig.Name,
			Pin:  newConfig.Pin,
			Type: rpiutils.PinInterrupt,
		})
	if err != nil {
		return nil, err
	}
	callbackID := C.setupInterrupt(pi.piID, C.int(bcom))
	if callbackID < 0 {
		err := rpiutils.ConvertErrorCodeToMessage(int(callbackID), "error")
		return nil, errors.Errorf("Unable to set up interrupt on pin %s: %s", newConfig.Name, err)
	}

	pi.interrupts[bcom] = &rpiInterrupt{
		interrupt:  d,
		callbackID: C.uint(callbackID),
	}

	return d, nil
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (pi *piPigpio) DigitalInterruptNames() []string {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	names := []string{}
	for _, rpiInterrupt := range pi.interrupts {
		names = append(names, rpiInterrupt.interrupt.Name())
	}
	return names
}

// DigitalInterruptByName returns a digital interrupt by name.
// NOTE: During board setup, if a digital interrupt has not been created
// for a pin, then this function will attempt to create one with the pin
// number as the name.
func (pi *piPigpio) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	d, ok := findInterruptByName(name, pi.interrupts)
	if !ok {
		if bcom, have := rpiutils.BroadcomPinFromHardwareLabel(name); have {
			if d, ok := pi.interrupts[bcom]; ok {
				return d.interrupt, nil
			}
			return pi.createNewInterrupt(
				rpiutils.PinConfig{
					Name: name,
					Pin:  name,
					Type: rpiutils.PinInterrupt,
				}, bcom)
		}
		return d, fmt.Errorf("interrupt %s does not exist", name)
	}
	return d, nil
}

var (
	lastTick = uint32(0)
	// the interrupt callback returns the time since boot in microseconds, but will wrap every ~72 minutes
	// we use the tickRollovers global variable to track each time this has occurred, and update the ticks for every active interrupt
	// we assume that uint64 will be large enough for us to not worry about the ticks overflowing further
	tickRollovers = 0
)

//export pigpioInterruptCallback
func pigpioInterruptCallback(gpio, level int, rawTick uint32) {
	if rawTick < lastTick {
		tickRollovers++
	}
	lastTick = rawTick

	// tick is the time since the hardware was started in microseconds.
	tick := (uint64(tickRollovers) * uint64(math.MaxUint32)) + uint64(rawTick)

	// global lock to prevent multiple pins from interacting with the board
	boardInstanceMu.RLock()
	defer boardInstanceMu.RUnlock()

	// boardInstance has to be initialized before callback can be called
	if boardInstance == nil {
		return
	}
	interrupt := boardInstance.interrupts[uint(gpio)]
	if interrupt == nil {
		boardInstance.logger.Infof("no DigitalInterrupt configured for gpio %d", gpio)
		return
	}
	if interrupt.debounceMicroSeconds != 0 && tick-interrupt.lastTicks < interrupt.debounceMicroSeconds {
		// we have not passed the debounce time, ignore this interrupt
		return
	}
	high := true
	if level == 0 {
		high = false
	}
	switch di := interrupt.interrupt.(type) {
	case *rpiutils.BasicDigitalInterrupt:
		err := rpiutils.Tick(boardInstance.cancelCtx, di, high, tick*1000)
		if err != nil {
			boardInstance.logger.Error(err)
		}
	default:
		boardInstance.logger.Error("unknown digital interrupt type")
	}
	// store the current ticks for debouncing
	interrupt.lastTicks = tick
}
