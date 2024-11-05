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
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/multierr"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
	rpiutils "raspberry-pi/utils"
)

// Model represents a raspberry pi board model.
var Model = resource.NewModel("viam", "raspberry-pi", "rpi")

var (
	boardInstance   *piPigpio    // global instance of raspberry pi borad for interrupt callbacks
	boardInstanceMu sync.RWMutex // mutex to protect boardInstance
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	Pins          []rpiutils.PinConfig                `json:"pins,omitempty"`
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

	for _, c := range conf.Pins {
		if err := c.Validate(path); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	resource.Named

	mu            sync.Mutex
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	gpioPins      map[int]*rpiGPIO
	analogReaders map[string]*pinwrappers.AnalogSmoother
	// `interrupts` maps interrupt names to the interrupts. `interruptsHW` maps broadcom addresses
	// to these same values. The two should always have the same set of values.
	interrupts map[uint]*rpiInterrupt
	logger     logging.Logger
	isClosed   bool

	piID C.int // id to communicate with pigpio daemon

	pulls map[int]string // mapping of gpio pin to pull up/down

	activeBackgroundWorkers sync.WaitGroup
}

var (
	pigpioInitialized bool
	// To prevent deadlocks, we must never lock the mutex of a specific piPigpio struct, above,
	// while this is locked. It is okay to lock this while one of those other mutexes is locked
	// instead.
	instanceMu      sync.RWMutex
	instances       = map[*piPigpio]struct{}{}
	daemonBootDelay = time.Duration(50) * time.Millisecond
)

// newPigpio makes a new pigpio based Board using the given config.
func newPigpio(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (board.Board, error) {
	err := startPigpiod(ctx, logger)
	if err != nil {
		logger.CErrorf(ctx, "Failed to start pigpiod: %v", err)
		return nil, err
	}
	time.Sleep(daemonBootDelay)
	piID, err := initializePigpio()
	if err != nil {
		return nil, err
	}
	logger.CInfo(ctx, "successfully started pigpiod")

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
		C.pigpio_stop(piID)
		logger.CError(ctx, "Pi GPIO terminated due to failed init.")
		return nil, err
	}

	return piInstance, nil
}

// Function initializes connection to pigpio daemon.
func initializePigpio() (C.int, error) {
	boardInstanceMu.Lock()
	defer boardInstanceMu.Unlock()

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
	// make sure every pin has a name. We already know every pin has a pin
	for _, c := range cfg.Pins {
		if c.Name == "" {
			c.Name = c.Pin
		}
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	if err := pi.reconfigureAnalogReaders(cfg); err != nil {
		return err
	}

	if err := pi.reconfigureGPIOs(ctx, cfg); err != nil {
		return err
	}

	// This is the only one that actually uses ctx, but we pass it to all previous helpers, too, to
	// keep the interface consistent.
	if err := pi.reconfigureInterrupts(ctx, cfg); err != nil {
		return err
	}

	if err := pi.reconfigurePulls(ctx, cfg); err != nil {
		return err
	}

	boardInstanceMu.Lock()
	defer boardInstanceMu.Unlock()
	boardInstance = pi

	return nil
}

func (pi *piPigpio) reconfigurePulls(ctx context.Context, cfg *Config) error {
	for _, pullConf := range cfg.Pins {
		// skip pins that do not have a pull state set
		if pullConf.PullState == rpiutils.PullDefault {
			continue
		}
		gpioNum, have := rpiutils.BroadcomPinFromHardwareLabel(pullConf.Pin)
		if !have {
			return fmt.Errorf("error configuring pull: no gpio pin found for %s", pullConf.Name)
		}
		switch pullConf.PullState {
		case rpiutils.PullNone:
			if result := C.setPullNone(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		case rpiutils.PullUp:
			if result := C.setPullUp(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		case rpiutils.PullDown:
			if result := C.setPullDown(pi.piID, C.int(gpioNum)); result != 0 {
				pi.logger.Error(rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
			}
		default:
			return fmt.Errorf("error configuring gpio pin %v pull: unexpected pull method %v", pullConf.Name, pullConf.PullState)
		}

	}
	return nil
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

	boardInstanceMu.Lock()
	boardInstance = nil
	boardInstanceMu.Unlock()
	// TODO: test this with multiple instences of the board.
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
	for _, rpiInterrupt := range pi.interrupts {
		if result := C.teardownInterrupt(rpiInterrupt.callbackID); result != 0 {
			err = multierr.Combine(err, rpiutils.ConvertErrorCodeToMessage(int(result), "error"))
		}
	}
	pi.interrupts = map[uint]*rpiInterrupt{}
	return err
}
