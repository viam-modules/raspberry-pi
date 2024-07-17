// Package rpi implements raspberry pi board
package rpi

/*
	This driver contains various functionalities of raspberry pi board using the
	pigpio daemon library (https://abyz.me.uk/rpi/pigpio/pdif2.html).
	NOTE: This driver only supports software PWM functionality of raspberry pi.
		  For software PWM, we currently support the default sample rate of
		  5 microseconds, which supports the following 18 frequencies (Hz):
		  8000  4000  2000 1600 1000  800  500  400  320
		  250   200   160  100   80   50   40   20   10
		  Details on this can be found here -> https://abyz.me.uk/rpi/pigpio/pdif2.html#set_PWM_frequency
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpiod_if2
import "C"

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("viam", "raspberry-pi", "rpi")

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders     []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	DigitalInterrupts []DigitalInterruptConfig            `json:"digital_interrupts,omitempty"`
}

// init registers a pi board based on pigpio.
func init() {
	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[board.Board, *Config]{
			Constructor: newPigpio,
		})
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, c := range conf.AnalogReaders {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, c := range conf.DigitalInterrupts {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	resource.Named
	// To prevent deadlocks, we must never lock this mutex while instanceMu, defined below, is
	// locked. It's okay to lock instanceMu while this is locked, though. This invariant prevents
	// deadlocks if both mutexes are locked by separate goroutines and are each waiting to lock the
	// other as well.
	mu            sync.Mutex
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	duty          int // added for mutex
	gpioConfigSet map[int]bool
	analogReaders map[string]*pinwrappers.AnalogSmoother
	// `interrupts` maps interrupt names to the interrupts. `interruptsHW` maps broadcom addresses
	// to these same values. The two should always have the same set of values.
	interrupts   map[string]rpiutils.ReconfigurableDigitalInterrupt
	interruptsHW map[uint]rpiutils.ReconfigurableDigitalInterrupt
	logger       logging.Logger
	isClosed     bool

	piID int

	activeBackgroundWorkers sync.WaitGroup
}

var (
	pigpioInitialized bool
	// To prevent deadlocks, we must never lock the mutex of a specific piPigpio struct, above,
	// while this is locked. It is okay to lock this while one of those other mutexes is locked
	// instead.
	instanceMu sync.RWMutex
	instances  = map[*piPigpio]struct{}{}
)

// newPigpio makes a new pigpio based Board using the given config.
func newPigpio(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (board.Board, error) {
	piID, err := initializePigpio()
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	piInstance := &piPigpio{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		isClosed:   false,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		piID:       piID,
	}

	if err := piInstance.Reconfigure(ctx, nil, conf); err != nil {
		// This has to happen outside of the lock to avoid a deadlock with interrupts.
		C.pigpio_stop(C.int(piID))
		instanceMu.Lock()
		pigpioInitialized = false
		instanceMu.Unlock()
		logger.CError(ctx, "Pi GPIO terminated due to failed init.")
		return nil, err
	}

	return piInstance, nil
}

func (pi *piPigpio) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	if err := pi.reconfigureAnalogReaders(ctx, cfg); err != nil {
		return err
	}

	// This is the only one that actually uses ctx, but we pass it to all previous helpers, too, to
	// keep the interface consistent.
	if err := pi.reconfigureInterrupts(ctx, cfg); err != nil {
		return err
	}

	instanceMu.Lock()
	defer instanceMu.Unlock()
	instances[pi] = struct{}{}
	return nil
}

func initializePigpio() (int, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if pigpioInitialized {
		return -1, nil
	}

	piID := C.custom_pigpio_start()
	if piID < 0 {
		// failed to init, check for common causes
		_, err := os.Stat("/sys/bus/platform/drivers/raspberrypi-firmware")
		if err != nil {
			return -1, errors.New("not running on a pi")
		}
		if os.Getuid() != 0 {
			return -1, errors.New("not running as root, try sudo")
		}
		return -1, rpiutils.ConvertErrorCodeToMessage(int(piID), "error")
	}

	pigpioInitialized = true
	return int(piID), nil
}

func (pi *piPigpio) reconfigureAnalogReaders(ctx context.Context, cfg *Config) error {
	// No need to reconfigure the old analog readers; just throw them out and make new ones.
	pi.analogReaders = map[string]*pinwrappers.AnalogSmoother{}
	for _, ac := range cfg.AnalogReaders {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		bus := &piPigpioSPI{pi: pi, busSelect: ac.SPIBus}
		ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, ac.ChipSelect}

		pi.analogReaders[ac.Name] = pinwrappers.SmoothAnalogReader(ar, board.AnalogReaderConfig{
			AverageOverMillis: ac.AverageOverMillis, SamplesPerSecond: ac.SamplesPerSecond,
		}, pi.logger)
	}
	return nil
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
			if result := C.teardownInterrupt(C.int(pi.piID), C.int(oldBcom)); result != 0 {
				return picommon.ConvertErrorCodeToMessage(int(result), "error")
			}
		} else {
			// This should never happen, either, but is similarly not really a problem.
			pi.logger.CErrorf(ctx,
				"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
					"but couldn't find its old bcom!?", name, bcom)
		}

		if result := C.setupInterrupt(C.int(pi.piID), C.int(bcom)); result != 0 {
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
		di, err := CreateDigitalInterrupt(newConfig)
		if err != nil {
			return err
		}
		newInterrupts[newConfig.Name] = di
		newInterruptsHW[bcom] = di
		if result := C.setupInterrupt(C.int(pi.piID), C.int(bcom)); result != 0 {
			return picommon.ConvertErrorCodeToMessage(int(result), "error")
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
			if result := C.teardownInterrupt(C.int(pi.piID), C.int(bcom)); result != 0 {
				return picommon.ConvertErrorCodeToMessage(int(result), "error")
			}
		}
	}

	pi.interrupts = newInterrupts
	pi.interruptsHW = newInterruptsHW
	return nil
}
