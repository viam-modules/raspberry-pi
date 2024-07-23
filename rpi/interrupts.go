package rpi

/*
	This file implements digital interrupt functionality for the Raspberry Pi.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"
	"math"

	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
)

// This is a helper function for digital interrupt reconfiguration. It finds the key in the map
// whose value is the given interrupt, and returns that key and whether we successfully found it.
func findInterruptName(
	interrupt rpiutils.ReconfigurableDigitalInterrupt,
	interrupts map[string]rpiutils.ReconfigurableDigitalInterrupt,
) (string, bool) {
	for key, value := range interrupts {
		if value == interrupt {
			return key, true
		}
	}
	return "", false
}

// This is a very similar helper function, which does the same thing but for broadcom addresses.
func findInterruptBcom(
	interrupt rpiutils.ReconfigurableDigitalInterrupt,
	interruptsHW map[uint]rpiutils.ReconfigurableDigitalInterrupt,
) (uint, bool) {
	for key, value := range interruptsHW {
		if value == interrupt {
			return key, true
		}
	}
	return 0, false
}

func (pi *piPigpio) reconfigureInterrupts(ctx context.Context, cfg *Config) error {
	// We reuse the old interrupts when possible.
	oldInterrupts := pi.interrupts
	oldInterruptsHW := pi.interruptsHW
	// Like with pi.interrupts and pi.interruptsHW, these two will have identical values, mapped to
	// using different keys.
	newInterrupts := map[string]rpiutils.ReconfigurableDigitalInterrupt{}
	newInterruptsHW := map[uint]rpiutils.ReconfigurableDigitalInterrupt{}

	// This begins as a set of all interrupts, but we'll remove the ones we reuse. Then, we'll
	// close whatever is left over.
	interruptsToClose := make(
		map[rpiutils.ReconfigurableDigitalInterrupt]struct{},
		len(oldInterrupts),
	)

	for _, interrupt := range oldInterrupts {
		interruptsToClose[interrupt] = struct{}{}
	}

	reuseInterrupt := func(
		interrupt rpiutils.ReconfigurableDigitalInterrupt,
		name string,
		bcom uint,
	) error {
		newInterrupts[name] = interrupt
		newInterruptsHW[bcom] = interrupt
		delete(interruptsToClose, interrupt)

		// We also need to remove the reused interrupt from oldInterrupts and oldInterruptsHW, to
		// avoid double-reuse (e.g., the old interrupt had name "foo" on pin 7, and the new config
		// has name "foo" on pin 8 and name "bar" on pin 7).
		if oldName, ok := findInterruptName(interrupt, oldInterrupts); ok {
			delete(oldInterrupts, oldName)
		} else {
			// This should never happen. However, if it does, nothing is obviously broken, so we'll
			// just log the weirdness and continue.
			pi.logger.CErrorf(ctx,
				"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
					"but couldn't find its old name!?", name, bcom)
		}

		if oldBcom, ok := findInterruptBcom(interrupt, oldInterruptsHW); ok {
			delete(oldInterruptsHW, oldBcom)
			if result := C.teardownInterrupt(pi.piID, C.int(oldBcom)); result != 0 {
				return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
			}
		} else {
			// This should never happen, either, but is similarly not really a problem.
			pi.logger.CErrorf(ctx,
				"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
					"but couldn't find its old bcom!?", name, bcom)
		}

		if result := C.setupInterrupt(pi.piID, C.int(bcom)); result != 0 {
			return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
		}
		return nil
	}

	for _, newConfig := range cfg.DigitalInterrupts {
		bcom, ok := rpiutils.BroadcomPinFromHardwareLabel(newConfig.Pin)
		if !ok {
			return errors.Errorf("no hw mapping for %s", newConfig.Pin)
		}

		// Try reusing an interrupt with the same pin
		if oldInterrupt, ok := oldInterruptsHW[bcom]; ok {
			if err := reuseInterrupt(oldInterrupt, newConfig.Name, bcom); err != nil {
				return err
			}
			continue
		}
		// If that didn't work, try reusing an interrupt with the same name
		if oldInterrupt, ok := oldInterrupts[newConfig.Name]; ok {
			if err := reuseInterrupt(oldInterrupt, newConfig.Name, bcom); err != nil {
				return err
			}
			continue
		}

		// Otherwise, create the new interrupt from scratch.
		di, err := rpiutils.CreateDigitalInterrupt(newConfig)
		if err != nil {
			return err
		}
		newInterrupts[newConfig.Name] = di
		newInterruptsHW[bcom] = di
		if result := C.setupInterrupt(pi.piID, C.int(bcom)); result != 0 {
			return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
		}
	}

	// For the remaining interrupts, keep any that look implicitly created (interrupts whose name
	// matches its broadcom address), and get rid of the rest.
	for interrupt := range interruptsToClose {
		name, ok := findInterruptName(interrupt, oldInterrupts)
		if !ok {
			// This should never happen
			return errors.Errorf("Logic bug: found old interrupt %s without old name!?", interrupt)
		}

		bcom, ok := findInterruptBcom(interrupt, oldInterruptsHW)
		if !ok {
			// This should never happen, either
			return errors.Errorf("Logic bug: found old interrupt %s without old bcom!?", interrupt)
		}

		if expectedBcom, ok := rpiutils.BroadcomPinFromHardwareLabel(name); ok && bcom == expectedBcom {
			// This digital interrupt looks like it was implicitly created. Keep it around!
			newInterrupts[name] = interrupt
			newInterruptsHW[bcom] = interrupt
		} else {
			// This digital interrupt is no longer used.
			if result := C.teardownInterrupt(pi.piID, C.int(bcom)); result != 0 {
				return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
			}
		}
	}

	pi.interrupts = newInterrupts
	pi.interruptsHW = newInterruptsHW
	return nil
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (pi *piPigpio) DigitalInterruptNames() []string {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	names := []string{}
	for k := range pi.interrupts {
		names = append(names, k)
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
	d, ok := pi.interrupts[name]
	if !ok {
		var err error
		if bcom, have := rpiutils.BroadcomPinFromHardwareLabel(name); have {
			if d, ok := pi.interruptsHW[bcom]; ok {
				return d, nil
			}
			d, err = rpiutils.CreateDigitalInterrupt(
				rpiutils.DigitalInterruptConfig{
					Name: name,
					Pin:  name,
					Type: "basic",
				})
			if err != nil {
				return nil, err
			}
			if result := C.setupInterrupt(pi.piID, C.int(bcom)); result != 0 {
				err := rpiutils.ConvertErrorCodeToMessage(int(result), "error")
				return nil, errors.Errorf("Unable to set up interrupt on pin %s: %s", name, err)
			}

			pi.interrupts[name] = d
			pi.interruptsHW[bcom] = d
			return d, nil
		}
		return d, fmt.Errorf("interrupt %s does not exist", name)
	}
	return d, nil
}

var (
	lastTick      = uint32(0)
	tickRollevers = 0
)

//export pigpioInterruptCallback
func pigpioInterruptCallback(gpio, level int, rawTick uint32) {
	if rawTick < lastTick {
		tickRollevers++
	}
	lastTick = rawTick

	tick := (uint64(tickRollevers) * uint64(math.MaxUint32)) + uint64(rawTick)

	instanceMu.RLock()
	defer instanceMu.RUnlock()
	for instance := range instances {
		i := instance.interruptsHW[uint(gpio)]
		if i == nil {
			logging.Global().Infof("no DigitalInterrupt configured for gpio %d", gpio)
			continue
		}
		high := true
		if level == 0 {
			high = false
		}
		// this should *not* block for long otherwise the lock
		// will be held
		switch di := i.(type) {
		case *rpiutils.BasicDigitalInterrupt:
			err := rpiutils.Tick(instance.cancelCtx, di, high, tick*1000)
			if err != nil {
				instance.logger.Error(err)
			}
		case *rpiutils.ServoDigitalInterrupt:
			err := rpiutils.ServoTick(instance.cancelCtx, di, high, tick*1000)
			if err != nil {
				instance.logger.Error(err)
			}
		default:
			instance.logger.Error("unknown digital interrupt type")
		}
	}
}
