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
	"strings"
	"sync"
	"time"

	rpiutils "raspberry-pi/utils"

	"go.uber.org/multierr"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

// Model represents a raspberry pi board model.
var (
	ModelPi    = rpiutils.RaspiFamily.WithModel("rpi")    // Raspberry Pi Generic model
	ModelPi4   = rpiutils.RaspiFamily.WithModel("rpi4")   // Raspberry Pi 4 model
	ModelPi3   = rpiutils.RaspiFamily.WithModel("rpi3")   // Raspberry Pi 3 model
	ModelPi2   = rpiutils.RaspiFamily.WithModel("rpi2")   // Raspberry Pi 2 model
	ModelPi1   = rpiutils.RaspiFamily.WithModel("rpi1")   // Raspberry Pi 1 model
	ModelPi0_2 = rpiutils.RaspiFamily.WithModel("rpi0_2") // Raspberry Pi 0_2 model
	ModelPi0   = rpiutils.RaspiFamily.WithModel("rpi0")   // Raspberry Pi 0 model
)

var (
	boardInstance   *piPigpio    // global instance of raspberry pi borad for interrupt callbacks
	boardInstanceMu sync.RWMutex // mutex to protect boardInstance
)

// init registers a pi board based on pigpio.
func init() {
	resource.RegisterComponent(
		board.API,
		ModelPi,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi4,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi3,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi2,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi1,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi0_2,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
	resource.RegisterComponent(
		board.API,
		ModelPi0,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: newPigpio,
		})
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	resource.Named
	model string

	mu            sync.Mutex
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	pinConfigs    []rpiutils.PinConfig
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
	piModel, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		logging.Global().Errorw("Cannot determine raspberry pi model", "error", err)
	}
	isPi5 := strings.Contains(string(piModel), "Raspberry Pi 5")
	if isPi5 {
		return nil, rpiutils.WrongModelErr(conf.Name)
	}

	err = startPigpiod(ctx, logger)
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
		model:      conf.Model.Name,
		interrupts: make(map[uint]*rpiInterrupt),
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
	cfg, err := resource.NativeConfig[*rpiutils.Config](conf)
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

	if err := pi.reconfigureGPIOs(cfg); err != nil {
		return err
	}

	// This is the only one that actually uses ctx, but we pass it to all previous helpers, too, to
	// keep the interface consistent.
	if err := pi.reconfigureInterrupts(cfg); err != nil {
		return err
	}

	if err := pi.reconfigurePulls(cfg); err != nil {
		return err
	}

	if err := pi.configureI2C(cfg); err != nil {
		return err
	}

	pi.pinConfigs = cfg.Pins

	boardInstanceMu.Lock()
	defer boardInstanceMu.Unlock()
	boardInstance = pi

	return nil
}

func (pi *piPigpio) reconfigurePulls(cfg *rpiutils.Config) error {
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

func (pi *piPigpio) configureI2C(cfg *rpiutils.Config) error {
	// Only enable I2C if turn_i2c_on is true, otherwise do nothing
	if !cfg.BoardSettings.TurnI2COn {
		return nil
	}

	var configChanged, moduleChanged bool
	var err error
	var configFailed, moduleFailed bool

	configChanged, err = pi.updateI2CConfig("on")
	if err != nil {
		pi.logger.Errorf("Failed to enable I2C in boot config: %v", err)
		configFailed = true
	}

	moduleChanged, err = pi.updateI2CModule(true)
	if err != nil {
		pi.logger.Errorf("Failed to enable I2C module: %v", err)
		moduleFailed = true
	}

	if configFailed || moduleFailed {
		pi.logger.Errorf("Automatic I2C configuration failed. Please manually enable I2C using 'sudo raspi-config' -> Interfacing Options -> I2C")
		return nil
	}

	if configChanged || moduleChanged {
		pi.logger.Infof("I2C configuration enabled. Initiating automatic reboot...")
		go rpiutils.PerformReboot(pi.logger)
	}

	return nil
}

func (pi *piPigpio) updateI2CConfig(desiredValue string) (bool, error) {
	configPath := rpiutils.GetBootConfigPath()
	return rpiutils.UpdateConfigFile(configPath, "dtparam=i2c_arm", desiredValue, pi.logger)
}

func (pi *piPigpio) updateI2CModule(enable bool) (bool, error) {
	return rpiutils.UpdateModuleFile("/etc/modules", "i2c-dev", enable, pi.logger)
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
