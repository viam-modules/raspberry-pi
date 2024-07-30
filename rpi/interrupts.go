package rpi

/*
	This file implements digital interrupt functionality for the Raspberry Pi.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
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

// reconfigureContext contains the context and state required for reconfiguring interrupts.
type reconfigureContext struct {
	pi  *piPigpio
	ctx context.Context

	// We reuse the old interrupts when possible.
	oldInterrupts   map[string]rpiutils.ReconfigurableDigitalInterrupt
	oldInterruptsHW map[uint]rpiutils.ReconfigurableDigitalInterrupt

	// Like oldInterrupts and oldInterruptsHW, these two will have identical values, mapped to
	// using different keys.
	newInterrupts   map[string]rpiutils.ReconfigurableDigitalInterrupt
	newInterruptsHW map[uint]rpiutils.ReconfigurableDigitalInterrupt

	interruptsToClose map[rpiutils.ReconfigurableDigitalInterrupt]struct{}
}

// reconfigureInterrupts reconfigures the digital interrupts based on the new configuration provided.
// It reuses existing interrupts when possible and creates new ones if necessary.
func (pi *piPigpio) reconfigureInterrupts(ctx context.Context, cfg *Config) error {
	reconfigCtx := &reconfigureContext{
		pi:                pi,
		ctx:               ctx,
		oldInterrupts:     pi.interrupts,
		oldInterruptsHW:   pi.interruptsHW,
		newInterrupts:     make(map[string]rpiutils.ReconfigurableDigitalInterrupt),
		newInterruptsHW:   make(map[uint]rpiutils.ReconfigurableDigitalInterrupt),
		interruptsToClose: initializeInterruptsToClose(pi.interrupts),
	}

	for _, newConfig := range cfg.DigitalInterrupts {
		bcom, ok := rpiutils.BroadcomPinFromHardwareLabel(newConfig.Pin)
		if !ok {
			return errors.Errorf("no hw mapping for %s", newConfig.Pin)
		}

		if err := reconfigCtx.tryReuseOrCreateInterrupt(newConfig, bcom); err != nil {
			return err
		}
	}

	if err := reconfigCtx.cleanupUnusedInterrupts(); err != nil {
		return err
	}

	pi.interrupts = reconfigCtx.newInterrupts
	pi.interruptsHW = reconfigCtx.newInterruptsHW
	return nil
}

// type aliases for initializeInterruptsToClose function
type InterruptMap map[string]rpiutils.ReconfigurableDigitalInterrupt
type InterruptSet map[rpiutils.ReconfigurableDigitalInterrupt]struct{}

// initializeInterruptsToClose initializes a map of interrupts to be closed by adding all old interrupts to it.
func initializeInterruptsToClose(oldInterrupts InterruptMap) InterruptSet {
	interruptsToClose := make(map[rpiutils.ReconfigurableDigitalInterrupt]struct{}, len(oldInterrupts))
	for _, interrupt := range oldInterrupts {
		interruptsToClose[interrupt] = struct{}{}
	}
	return interruptsToClose
}

// tryReuseOrCreateInterrupt attempts to reuse an existing interrupt or create a new one if no reusable interrupt is found.
// It tries to reuse an interrupt by its hardware pin or name, and if both fail, it creates a new interrupt.
func (ctx *reconfigureContext) tryReuseOrCreateInterrupt(newConfig rpiutils.DigitalInterruptConfig, bcom uint) error {
	if oldInterrupt, ok := ctx.oldInterruptsHW[bcom]; ok {
		return ctx.reuseInterrupt(oldInterrupt, newConfig.Name, bcom)
	}

	if oldInterrupt, ok := ctx.oldInterrupts[newConfig.Name]; ok {
		return ctx.reuseInterrupt(oldInterrupt, newConfig.Name, bcom)
	}

	return ctx.createNewInterrupt(newConfig, bcom)
}

// reuseInterrupt reconfigures an existing interrupt for reuse and updates the necessary maps and configurations.
// It removes the reused interrupt from the list of interrupts to be closed and updates the new interrupt maps.
func (ctx *reconfigureContext) reuseInterrupt(interrupt rpiutils.ReconfigurableDigitalInterrupt, name string, bcom uint) error {
	ctx.newInterrupts[name] = interrupt
	ctx.newInterruptsHW[bcom] = interrupt
	delete(ctx.interruptsToClose, interrupt)

	if oldName, ok := findInterruptName(interrupt, ctx.oldInterrupts); ok {
		delete(ctx.oldInterrupts, oldName)
	} else {
		ctx.pi.logger.CErrorf(ctx.ctx,
			"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
				"but couldn't find its old name!?", name, bcom)
	}

	if oldBcom, ok := findInterruptBcom(interrupt, ctx.oldInterruptsHW); ok {
		delete(ctx.oldInterruptsHW, oldBcom)
		if result := C.teardownInterrupt(ctx.pi.piID, C.int(oldBcom)); result != 0 {
			return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
		}
	} else {
		ctx.pi.logger.CErrorf(ctx.ctx,
			"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
				"but couldn't find its old bcom!?", name, bcom)
	}

	if result := C.setupInterrupt(ctx.pi.piID, C.int(bcom)); result != 0 {
		return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
	}
	return nil
}

// createNewInterrupt creates a new digital interrupt and sets it up with the specified configuration.
func (ctx *reconfigureContext) createNewInterrupt(newConfig rpiutils.DigitalInterruptConfig, bcom uint) error {
	di, err := rpiutils.CreateDigitalInterrupt(newConfig)
	if err != nil {
		return err
	}
	ctx.newInterrupts[newConfig.Name] = di
	ctx.newInterruptsHW[bcom] = di
	if result := C.setupInterrupt(ctx.pi.piID, C.int(bcom)); result != 0 {
		return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
	}
	return nil
}

// createNewInterrupt creates a new digital interrupt and sets it up with the specified configuration.
// It adds the new interrupt to the new interrupt maps and sets up the interrupt in the hardware.
func (ctx *reconfigureContext) cleanupUnusedInterrupts() error {
	for interrupt := range ctx.interruptsToClose {
		name, ok := findInterruptName(interrupt, ctx.oldInterrupts)
		if !ok {
			return errors.Errorf("found old interrupt %s without old name!?", interrupt)
		}

		bcom, ok := findInterruptBcom(interrupt, ctx.oldInterruptsHW)
		if !ok {
			return errors.Errorf("found old interrupt %s without old bcom!?", interrupt)
		}

		if expectedBcom, ok := rpiutils.BroadcomPinFromHardwareLabel(name); ok && bcom == expectedBcom {
			ctx.newInterrupts[name] = interrupt
			ctx.newInterruptsHW[bcom] = interrupt
		} else {
			if result := C.teardownInterrupt(ctx.pi.piID, C.int(bcom)); result != 0 {
				return rpiutils.ConvertErrorCodeToMessage(int(result), "error")
			}
		}
	}
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
