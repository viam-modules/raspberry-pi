//go:build linux

// Package pi5 implements a raspberry pi5 board using pinctrl
package pi5

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	rpiutils "raspberry-pi/utils"

	"github.com/pkg/errors"
	"github.com/viam-modules/pinctrl/pinctrl"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/rdk/components/board"
	gl "go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

// Model for rpi5.
var Model = resource.NewModel("viam", "raspberry-pi", "rpi5")

// pins are stored in /dev/gpiomem in order of gpio nums, so we must convert from pin name (physical num) to GPIO number.
var pinNameToGPIONum = map[string]int{
	"3":  2,
	"5":  3,
	"7":  4,
	"8":  14,
	"10": 15,
	"11": 17,
	"12": 18,
	"13": 27,
	"15": 22,
	"16": 23,
	"18": 24,
	"19": 10,
	"21": 9,
	"22": 25,
	"23": 11,
	"24": 8,
	"26": 7,
	"27": 0,
	"28": 1,
	"29": 5,
	"31": 6,
	"32": 12,
	"33": 13,
	"35": 19,
	"36": 16,
	"37": 26,
	"38": 20,
	"40": 21,
}

// register values for configuring pull up/pull down in mem.
const (
	pullNoneMode = 0x0
	pullDownMode = 0x4
	pullUpMode   = 0x8
)

func init() {
	gpioMappings, err := gl.GetGPIOBoardMappings(Model.Name, boardInfoMappings)
	var noBoardErr gl.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		logging.Global().Debugw("Error getting raspi5 GPIO board mapping", "error", err)
	}

	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[board.Board, *rpiutils.Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (board.Board, error) {
				return newBoard(ctx, conf, gpioMappings, logger, false)
			},
		})
}

// newBoard is the constructor for a Board.
func newBoard(
	ctx context.Context,
	conf resource.Config,
	gpioMappings map[string]gl.GPIOBoardMapping,
	logger logging.Logger,
	testingMode bool,
) (board.Board, error) {
	var err error
	piModel, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		logging.Global().Errorw("Cannot determine raspberry pi model", "error", err)
	}
	isPi5 := strings.Contains(string(piModel), "5")
	if !isPi5 {
		return nil, rpiutils.WrongModelErr(conf.Name)
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	b := &pinctrlpi5{
		Named: conf.ResourceName().AsNamed(),

		gpioMappings: gpioMappings,
		logger:       logger,
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,

		gpios:      map[string]*pinctrl.GPIOPin{},
		interrupts: map[string]*pinctrl.DigitalInterrupt{},

		pulls: map[int]byte{},
	}

	pinctrlCfg := pinctrl.Config{
		GPIOChipPath: "gpio0", DevMemPath: "/dev/gpiomem0",
		ChipSize: 0x30000, UseAlias: true, UseGPIOMem: true,
	}
	if testingMode {
		pinctrlCfg.TestPath = "./mock-device-tree"
	}

	// Note that this must be called before configuring the pull up/down configuration uses the
	// memory mapped in this function.
	b.boardPinCtrl, err = pinctrl.SetupPinControl(pinctrlCfg, logger)
	if err != nil {
		return nil, err
	}

	// Initialize the GPIO pins
	for newName, mapping := range gpioMappings {
		b.gpios[newName] = b.boardPinCtrl.CreateGpioPin(mapping)
	}

	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *pinctrlpi5) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*rpiutils.Config](conf)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.reconfigurePullUpPullDowns(newConf); err != nil {
		return err
	}
	return nil
}

func (b *pinctrlpi5) reconfigurePullUpPullDowns(newConf *rpiutils.Config) error {
	for _, pullConf := range newConf.Pins {
		gpioNum := pinNameToGPIONum[pullConf.Pin]

		switch pullConf.PullState {
		case rpiutils.PullDefault: // skip pins that do not have a pull state set
			continue
		case rpiutils.PullNone:
			b.pulls[gpioNum] = pullNoneMode
		case rpiutils.PullUp:
			b.pulls[gpioNum] = pullUpMode
		case rpiutils.PullDown:
			b.pulls[gpioNum] = pullDownMode
		default:
			return fmt.Errorf("error configuring gpio pin %v pull: unexpected pull method %v", pullConf.Name, pullConf.PullState)
		}
	}
	b.setPulls()

	return nil
}

// setPull is a helper function to access memory to set a pull up/pull down resisitor on a pin.
func (b *pinctrlpi5) setPulls() {
	// offset to the pads address space in /dev/gpiomem0
	// all gpio pins are in bank0
	PadsBank0Offset := 0x00020000

	for pin, mode := range b.pulls {
		// each pad has 4 header bytes + 4 bytes of memory for each gpio pin
		pinOffsetBytes := 4 + 4*pin

		// only the 5th and 6th bits of the register are used to set pull up/down
		// reset the register then set the mode
		b.boardPinCtrl.VPage[PadsBank0Offset+pinOffsetBytes] = (b.boardPinCtrl.VPage[PadsBank0Offset+pinOffsetBytes] & 0xf3) | mode
	}
}

type pinctrlpi5 struct {
	resource.Named
	mu sync.Mutex

	gpioMappings map[string]gl.GPIOBoardMapping
	logger       logging.Logger

	gpios      map[string]*pinctrl.GPIOPin
	interrupts map[string]*pinctrl.DigitalInterrupt

	boardPinCtrl pinctrl.Pinctrl

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	pulls map[int]byte // mapping of gpio pin to pull up/down
}

// AnalogByName returns the analog pin by the given name if it exists.
func (b *pinctrlpi5) AnalogByName(name string) (board.Analog, error) {
	return nil, errors.New("analogs not supported")
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *pinctrlpi5) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	interrupt, ok := b.interrupts[name]
	if ok {
		return interrupt, nil
	}

	// Otherwise, the name is not something we recognize yet. If it appears to be a GPIO pin, we'll
	// remove its GPIO capabilities and turn it into a digital interrupt.
	gpio, ok := b.gpios[name]
	if !ok {
		return nil, fmt.Errorf("can't find GPIO (%s)", name)
	}
	if err := gpio.Close(); err != nil {
		return nil, err
	}

	mapping, ok := b.gpioMappings[name]
	if !ok {
		return nil, fmt.Errorf("can't create digital interrupt on unknown pin %s", name)
	}
	defaultInterruptConfig := board.DigitalInterruptConfig{
		Name: name,
		Pin:  name,
	}
	interrupt, err := b.boardPinCtrl.NewDigitalInterrupt(defaultInterruptConfig, mapping, nil)
	if err != nil {
		return nil, err
	}

	delete(b.gpios, name)
	b.interrupts[name] = interrupt
	return interrupt, nil
}

// AnalogNames returns the names of all known analog pins.
func (b *pinctrlpi5) AnalogNames() []string {
	return []string{}
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (b *pinctrlpi5) DigitalInterruptNames() []string {
	if b.interrupts == nil {
		return nil
	}

	names := []string{}
	for name := range b.interrupts {
		names = append(names, name)
	}
	return names
}

// GPIOPinByName returns a GPIOPin by name.
func (b *pinctrlpi5) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	if pin, ok := b.gpios[pinName]; ok {
		return pin, nil
	}

	// Check if pin is a digital interrupt: those can still be used as inputs.
	if interrupt, interruptOk := b.interrupts[pinName]; interruptOk {
		return interrupt, nil
	}

	return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", pinName)
}

// SetPowerMode sets the board to the given power mode. If provided,
// the board will exit the given power mode after the specified
// duration.
func (b *pinctrlpi5) SetPowerMode(
	ctx context.Context,
	mode pb.PowerMode,
	duration *time.Duration,
) error {
	return grpc.UnimplementedError
}

// StreamTicks starts a stream of digital interrupt ticks.
func (b *pinctrlpi5) StreamTicks(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
	extra map[string]interface{},
) error {
	var rawInterrupts []*pinctrl.DigitalInterrupt
	for _, i := range interrupts {
		raw, ok := i.(*pinctrl.DigitalInterrupt)
		if !ok {
			return errors.New("cannot stream ticks to an interrupt not associated with this board")
		}
		rawInterrupts = append(rawInterrupts, raw)
	}

	for _, i := range rawInterrupts {
		i.AddChannel(ch)
	}

	b.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		// Wait until it's time to shut down then remove callbacks.
		select {
		case <-ctx.Done():
		case <-b.cancelCtx.Done():
		}
		for _, i := range rawInterrupts {
			i.RemoveChannel(ch)
		}
	}, b.activeBackgroundWorkers.Done)

	return nil
}

// Close attempts to cleanly close each part of the board.
func (b *pinctrlpi5) Close(ctx context.Context) error {
	b.mu.Lock()
	err := b.boardPinCtrl.Close()
	if err != nil {
		return fmt.Errorf("trouble cleaning up pincontrol memory: %w", err)
	}
	b.cancelFunc()
	b.mu.Unlock()
	b.activeBackgroundWorkers.Wait()

	for _, pin := range b.gpios {
		err = multierr.Combine(err, pin.Close())
	}
	for _, interrupt := range b.interrupts {
		err = multierr.Combine(err, interrupt.Close())
	}
	return err
}
