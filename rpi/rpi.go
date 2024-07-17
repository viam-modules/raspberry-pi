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

import (
	"context"
	"fmt"
	"sync"

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
	interrupts   map[string]ReconfigurableDigitalInterrupt
	interruptsHW map[uint]ReconfigurableDigitalInterrupt
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
