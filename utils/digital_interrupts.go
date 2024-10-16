// Package rpiutils contains implementations for digital_interrupts here.
package rpiutils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
)

// PinConfig describes the configuration of a pin for the board.
type PinConfig struct {
	Name       string  `json:"name"`
	Pin        string  `json:"pin"`
	Type       PinType `json:"type,omitempty"`        // e.g. gpio, interrupt
	DebounceMS int     `json:"debounce_ms,omitempty"` // only used with interrupts
	PullState  Pull    `json:"pull,omitempty"`
}

// PinType defines the pin types we support.
type PinType string

const (
	// PinGPIO represents GPIO pins.
	PinGPIO PinType = "gpio"
	// PinInterrupt represents interrupt pins.
	PinInterrupt PinType = "interrupt"
)

// Pull defines the pins pull state(pull up vs pull down).
type Pull string

const (
	// PullUp is for pull ups.
	PullUp Pull = "up"
	// PullDown is for pull downs.
	PullDown Pull = "down"
	// PullNone is for no pulls.
	PullNone Pull = "none"
	// PullDefault is for if no pull was set.
	PullDefault Pull = ""
)

// Validate validates that the pull is a valid message.
func (pull Pull) Validate() error {
	switch pull {
	case PullDefault:
	case PullUp:
	case PullDown:
	case PullNone:
	default:
		return fmt.Errorf("invalid pull configuration %v, supported pull config attributes are up, down, and none", pull)
	}
	return nil
}

// Validate ensures all parts of the config are valid.
func (config *PinConfig) Validate(path string) error {
	if config.Name == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Pin == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "pin")
	}
	return nil
}

// ServoRollingAverageWindow is how many entries to average over for
// servo ticks.
const ServoRollingAverageWindow = 10

// A ReconfigurableDigitalInterrupt is a simple reconfigurable digital interrupt that expects
// reconfiguration within the same type.
type ReconfigurableDigitalInterrupt interface {
	board.DigitalInterrupt
	Reconfigure(cfg PinConfig) error
}

// CreateDigitalInterrupt is a factory method for creating a specific DigitalInterrupt based
// on the given config. If no type is specified, an error is returned.
func CreateDigitalInterrupt(cfg PinConfig) (ReconfigurableDigitalInterrupt, error) {
	i := &BasicDigitalInterrupt{}
	//nolint:exhaustive
	switch cfg.Type {
	case PinInterrupt:
	default:
		return nil, fmt.Errorf("expected pin %v to be configured as %v, got %v instead", cfg.Name, PinInterrupt, cfg.Type)
	}

	if err := i.Reconfigure(cfg); err != nil {
		return nil, err
	}
	return i, nil
}

// A BasicDigitalInterrupt records how many ticks/interrupts happen and can
// report when they happen to interested callbacks.
type BasicDigitalInterrupt struct {
	count int64

	callbacks []chan board.Tick

	mu  sync.RWMutex
	cfg PinConfig
}

// Value returns the amount of ticks that have occurred.
func (i *BasicDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	count := atomic.LoadInt64(&i.count)
	return count, nil
}

// Tick records an interrupt and notifies any interested callbacks. See comment on
// the DigitalInterrupt interface for caveats.
func Tick(ctx context.Context, i *BasicDigitalInterrupt, high bool, nanoseconds uint64) error {
	if high {
		atomic.AddInt64(&i.count, 1)
	}

	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, c := range i.callbacks {
		select {
		case <-ctx.Done():
			return errors.New("context cancelled")
		case c <- board.Tick{Name: i.cfg.Name, High: high, TimestampNanosec: nanoseconds}:
		}
	}
	return nil
}

// AddCallback adds a listener for interrupts.
func AddCallback(i *BasicDigitalInterrupt, c chan board.Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.callbacks = append(i.callbacks, c)
}

// RemoveCallback removes a listener for interrupts.
func RemoveCallback(i *BasicDigitalInterrupt, c chan board.Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for id := range i.callbacks {
		if i.callbacks[id] == c {
			// To remove this item, we replace it with the last item in the list, then truncate the
			// list by 1.
			i.callbacks[id] = i.callbacks[len(i.callbacks)-1]
			i.callbacks = i.callbacks[:len(i.callbacks)-1]
			break
		}
	}
}

// Name returns the name of the interrupt.
func (i *BasicDigitalInterrupt) Name() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg.Name
}

// Reconfigure reconfigures this digital interrupt.
func (i *BasicDigitalInterrupt) Reconfigure(conf PinConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cfg = conf
	return nil
}
