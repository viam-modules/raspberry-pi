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
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	rpiutils "viamrpi/utils"

	pb "go.viam.com/api/component/board/v1"

	"go.uber.org/multierr"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

var Model = resource.NewModel("viam-hardware-testing", "raspberry-pi", "rpi")

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders     []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	DigitalInterrupts []rpiutils.DigitalInterruptConfig   `json:"digital_interrupts,omitempty"`
	Pulls             []PullConfig                        `json:"pulls,omitempty"`
}

// PullConfig defines the config for pull up/pull down resistors.
type PullConfig struct {
	Pin  string `json:"pin"`
	Pull string `json:"pull"`
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

	for _, c := range conf.Pulls {
		if c.Pin == "" {
			return nil, resource.NewConfigValidationFieldRequiredError(path, "pin")
		}
		if c.Pull == "" {
			return nil, resource.NewConfigValidationFieldRequiredError(path, "pull")
		}
		if !(c.Pull == "up" || c.Pull == "down" || c.Pull == "none") {
			return nil, fmt.Errorf("supported pull config attributes are up, down, and none")
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

	piID C.int

	pulls map[int]string // mapping of gpio pin to pull up/down

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

// Function initializes connection to pigpio daemon.
func initializePigpio() (C.int, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if pigpioInitialized {
		return -1, nil
	}

	piID := C.pigpio_start(nil, nil)
	if int(piID) < 0 {
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
	return piID, nil
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
	return nil
}

func (pi *piPigpio) ReconfigurePulls(ctx context.Context, cfg *Config) {
	for _, pullConf := range cfg.Pulls {
		gpioNum, have := rpiutils.BroadcomPinFromHardwareLabel(pullConf.Pin)
		if !have {
			pi.logger.Errorf("no gpio pin found for %s", pullConf.Pull)
			continue
		}
		switch pullConf.Pull {
		case "none":
			if result := C.setPullNone(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		case "up":
			if result := C.setPullUp(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		case "down":
			if result := C.setPullDown(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		default:
			pi.logger.Error("unexpected pull")
		}

	}

}

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close(ctx context.Context) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.isClosed {
		pi.logger.Info("Duplicate call to close pi board detected, skipping")
		return nil
	}

	pi.cancelFunc()
	pi.activeBackgroundWorkers.Wait()

	var err error
	err = multierr.Combine(err,
		closeAnalogReaders(ctx, pi),
		teardownInterrupts(pi))

	//TODO: test this with multiple instences of the board.
	C.pigpio_stop(pi.piID)
	pi.logger.CDebug(ctx, "Pi GPIO terminated properly.")

	pi.isClosed = true
	return err
}

// StreamTicks starts a stream of digital interrupt ticks.
func (pi *piPigpio) StreamTicks(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
	extra map[string]interface{},
) error {
	for _, i := range interrupts {
		rpiutils.AddCallback(i.(*rpiutils.BasicDigitalInterrupt), ch)
	}

	pi.activeBackgroundWorkers.Add(1)

	utils.ManagedGo(func() {
		// Wait until it's time to shut down then remove callbacks.
		select {
		case <-ctx.Done():
		case <-pi.cancelCtx.Done():
		}
		for _, i := range interrupts {
			rpiutils.RemoveCallback(i.(*rpiutils.BasicDigitalInterrupt), ch)
		}
	}, pi.activeBackgroundWorkers.Done)

	return nil
}

func (pi *piPigpio) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// closeAnalogReaders closes all analog readers associated with the board.
func closeAnalogReaders(ctx context.Context, pi *piPigpio) error {
	var err error
	for _, analog := range pi.analogReaders {
		err = multierr.Combine(err, analog.Close(ctx))
	}
	pi.analogReaders = map[string]*pinwrappers.AnalogSmoother{}
	return err
}

// teardownInterrupts removes all hardware interrupts and cleans up.
func teardownInterrupts(pi *piPigpio) error {
	var err error
	for bcom := range pi.interruptsHW {
		if result := C.teardownInterrupt(pi.piID, C.int(bcom)); result != 0 {
			err = multierr.Combine(err, rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
		}
	}
	pi.interrupts = map[string]rpiutils.ReconfigurableDigitalInterrupt{}
	pi.interruptsHW = map[uint]rpiutils.ReconfigurableDigitalInterrupt{}
	return err
}
