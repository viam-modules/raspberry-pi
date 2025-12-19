//go:build linux

// Package pi5 implements a raspberry pi5 board using pinctrl
package pi5

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	rpiutils "raspberry-pi/utils"
)

// Model is the model for a Raspberry Pi 5.
var Model = rpiutils.RaspiFamily.WithModel("rpi5")

// register values for configuring pull up/pull down in mem.
const (
	pullNoneMode = 0x0
	pullDownMode = 0x4
	pullUpMode   = 0x8
)

func init() {
	logger := logging.NewLogger("pi5.init")
	gpioMappings, err := gl.GetGPIOBoardMappings(Model.Name, boardInfoMappings, logger)
	var noBoardErr gl.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		logger.Debugw("Error getting raspi5 GPIO board mapping", "error", err)
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

type pinctrlpi5 struct {
	resource.Named
	mu sync.Mutex

	gpioMappings map[string]gl.GPIOBoardMapping
	logger       logging.Logger

	gpios            map[uint]*pinctrl.GPIOPin
	interrupts       map[uint]*pinctrl.DigitalInterrupt
	userDefinedNames map[string]uint // user defined pin names that map to a line/boardcom
	pinConfigs       []rpiutils.PinConfig

	boardPinCtrl pinctrl.Pinctrl

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	pulls map[int]byte // mapping of gpio pin to pull up/down
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
		logger.Errorw("Cannot determine raspberry pi model", "error", err)
	}
	isPi5 := strings.Contains(string(piModel), "Raspberry Pi 5")
	// ensure that we are a pi5 when not running tests
	if !isPi5 && !testingMode {
		return nil, rpiutils.WrongModelErr(conf.Name)
	}

	// Check for hardware PWM overlay in config.txt
	if err = checkHardwarePWMOverlayIsConfigured(); err != nil {
		logger.Warnf("%v", err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	b := &pinctrlpi5{
		Named: conf.ResourceName().AsNamed(),

		gpioMappings: gpioMappings,
		logger:       logger,
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,

		gpios:      map[uint]*pinctrl.GPIOPin{},
		interrupts: map[uint]*pinctrl.DigitalInterrupt{},

		pulls: map[int]byte{},
	}

	pinctrlCfg := pinctrl.Config{
		GPIOChipPath: "gpio0", DevMemPath: "/dev/gpiomem0",
		ChipSize: 0x30000, UseAlias: true, UseGPIOMem: true,
	}
	if testingMode {
		pinctrlCfg.TestPath = "./pi5/mock-device-tree"
	}

	// Note that this must be called before configuring the pull up/down configuration uses the
	// memory mapped in this function.
	b.boardPinCtrl, err = pinctrl.SetupPinControl(pinctrlCfg, logger)
	if err != nil {
		return nil, err
	}

	// Initialize the GPIO pins
	for newName, mapping := range gpioMappings {
		bcom, _ := rpiutils.BroadcomPinFromHardwareLabel(newName)
		b.gpios[bcom] = b.boardPinCtrl.CreateGpioPin(mapping, rpiutils.DefaultPWMFreqHz)
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

	// make sure every pin has a name. We already know every pin has a pin
	// possibly clean this up at a later date
	for _, c := range newConf.Pins {
		if c.Name == "" {
			c.Name = c.Pin
		}
	}

	if err := b.validatePins(newConf); err != nil {
		return err
	}

	if err := b.reconfigurePullUpPullDowns(newConf); err != nil {
		return err
	}
	if err := b.reconfigureInterrupts(newConf); err != nil {
		return err
	}

	b.configureI2C(newConf)

	b.configureBT(newConf)

	b.pinConfigs = newConf.Pins

	return nil
}

// reconfigureInterrupts reconfigures the digital interrupts based on the new configuration provided.
// It reuses existing interrupts when possible and creates new ones if necessary.
func (b *pinctrlpi5) reconfigureInterrupts(newConf *rpiutils.Config) error {
	// look at previous interrupt config, and see if we removed any
	for _, oldConfig := range b.pinConfigs {
		if oldConfig.Type != rpiutils.PinInterrupt {
			continue
		}
		sameInterrupt := false
		for _, newConfig := range newConf.Pins {
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
		// we no longer want this interrupt, so we will remove it and add back the pin's gpio functionality
		bcom, ok := rpiutils.BroadcomPinFromHardwareLabel(oldConfig.Pin)
		if !ok {
			return errors.Errorf("cannot find GPIO for unknown pin: %s", oldConfig.Name)
		}
		// this actually removes the interrupt
		interrupt, ok := b.interrupts[bcom]
		if ok {
			if err := interrupt.Close(); err != nil {
				return err
			}
			delete(b.interrupts, bcom)
		}

		// add back the gpio pin to make it available to the user
		b.gpios[bcom] = b.boardPinCtrl.CreateGpioPin(b.gpioMappings[oldConfig.Pin], rpiutils.DefaultPWMFreqHz)
	}
	// add any new interrupts. DigitalInterruptByName will create the interrupt only if we are not already managing it.
	for _, newConfig := range newConf.Pins {
		if newConfig.Type != rpiutils.PinInterrupt {
			continue
		}
		if _, err := b.digitalInterruptByName(newConfig.Name, newConfig.DebounceMS); err != nil {
			return err
		}
	}

	return nil
}

// record all custom pin names that the user has defined in the config for lookup.
func (b *pinctrlpi5) validatePins(newConf *rpiutils.Config) error {
	nameToPin := map[string]uint{}
	for _, pinConf := range newConf.Pins {
		// ensure the configured pin is a real pin
		pin, ok := b.gpioMappings[pinConf.Pin]
		if !ok {
			return fmt.Errorf("pin %v could not be found", pinConf.Pin)
		}
		// check if the pin name matches a name we handle by default
		_, alreadyDefined := rpiutils.BroadcomPinFromHardwareLabel(pinConf.Name)
		if alreadyDefined {
			continue
		}
		// add the new name to our list of names to track
		nameToPin[pinConf.Name] = uint(pin.GPIO)
	}
	b.userDefinedNames = nameToPin
	return nil
}

func (b *pinctrlpi5) reconfigurePullUpPullDowns(newConf *rpiutils.Config) error {
	for _, pullConf := range newConf.Pins {
		pin, ok := b.gpioMappings[pullConf.Pin]
		if !ok {
			return fmt.Errorf("pin %v could not be found", pullConf.Pin)
		}
		gpioNum := pin.GPIO
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

// AnalogByName returns the analog pin by the given name if it exists.
func (b *pinctrlpi5) AnalogByName(name string) (board.Analog, error) {
	return nil, errors.New("analogs not supported")
}

// the implementation of digitalInterruptByName. The board mutex should be locked before calling this.
func (b *pinctrlpi5) digitalInterruptByName(name string, debounceMilliSeconds int) (board.DigitalInterrupt, error) {
	// first check if the pinName is a user defined name
	bcom, ok := b.userDefinedNames[name]
	if !ok {
		// if the name is not a user defined name, then check if its a known pin
		bcom, ok = rpiutils.BroadcomPinFromHardwareLabel(name)
		if !ok {
			return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", name)
		}
	}

	// if we are already managing the interrupt, then return the interrupt
	interrupt, ok := b.interrupts[bcom]
	if ok {
		return interrupt, nil
	}

	// Otherwise, the name is not something we recognize yet. If it appears to be a GPIO pin, we'll
	// remove its GPIO capabilities and turn it into a digital interrupt.
	gpio, ok := b.gpios[bcom]
	if !ok {
		return nil, fmt.Errorf("can't find GPIO (%s)", name)
	}
	if err := gpio.Close(); err != nil {
		return nil, err
	}

	hardwareName := ""
	var pinMapping gl.GPIOBoardMapping
	// When creating a new interrupt we need to pass in the genericlinux pin mapping.
	// Unfortunately with the bcom logic it ended up hard to track the generic linux pinmapping with the bcom number
	// to workaround this we have to run through all of the pinmappings to find which mapping is actually the requested version
	for newName, mapping := range b.gpioMappings {
		if mapping.GPIO == int(bcom) {
			hardwareName = newName
			pinMapping = mapping
		}
	}

	defaultInterruptConfig := board.DigitalInterruptConfig{
		Name: hardwareName,
		Pin:  hardwareName,
	}
	interrupt, err := b.boardPinCtrl.NewDigitalInterrupt(defaultInterruptConfig, pinMapping, debounceMilliSeconds, nil)
	if err != nil {
		return nil, err
	}

	delete(b.gpios, bcom)
	b.interrupts[bcom] = interrupt
	return interrupt, nil
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *pinctrlpi5) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.digitalInterruptByName(name, 0)
}

// AnalogNames returns the names of all known analog pins.
func (b *pinctrlpi5) AnalogNames() []string {
	return []string{}
}

// DigitalInterruptNames returns the names of all known digital interrupts.
// Unimplemented because we do not have an api to communicate this over.
func (b *pinctrlpi5) DigitalInterruptNames() []string {
	return nil
}

// GPIOPinByName returns a GPIOPin by name.
func (b *pinctrlpi5) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	// first check if the pinName is a user defined name
	bcom, ok := b.userDefinedNames[pinName]
	if !ok {
		// if the name is not a user defined name, then check if its a known pin
		bcom, ok = rpiutils.BroadcomPinFromHardwareLabel(pinName)
		if !ok {
			return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", pinName)
		}
	}

	// check if the pin is being managed as a gpio
	if pin, ok := b.gpios[bcom]; ok {
		return pin, nil
	}

	// Check if pin is a digital interrupt: those can still be used as inputs.
	if interrupt, interruptOk := b.interrupts[bcom]; interruptOk {
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

func (b *pinctrlpi5) configureBT(cfg *rpiutils.Config) {
	configPath := rpiutils.GetBootConfigPath()

	var (
		configChanged = false
		configFailed  = false
	)

	b.logger.Debugf("Bluetooth parameter configuration starting...")
	if cfg.BoardSettings.BTenableuart != nil {
		changed, failed := b.updateBTenableuart(configPath, *cfg.BoardSettings.BTenableuart)
		configChanged = configChanged || changed
		configFailed = configFailed || failed
	}

	if cfg.BoardSettings.BTdtoverlay != nil {
		changed, failed := b.updateBTminiuart(configPath, *cfg.BoardSettings.BTdtoverlay)
		configChanged = configChanged || changed
		configFailed = configFailed || failed
	}

	if cfg.BoardSettings.BTkbaudrate != nil {
		changed, failed := b.updateBTbaudrate(configPath, *cfg.BoardSettings.BTkbaudrate)
		configChanged = configChanged || changed
		configFailed = configFailed || failed
	}

	if configFailed {
		b.logger.Errorf("Automatic Bluetooth configuration failed. Please manually edit config.txt")
		return
	}

	if configChanged {
		b.logger.Infof("Bluetooth configuration modified. Initiating automatic reboot...")
		go rpiutils.PerformReboot(b.logger)
	}
}

// updateBTenableuart ensures either enable_uart=1 or enable_uart=0 is set, and the opposite is removed.
func (b *pinctrlpi5) updateBTenableuart(configPath string, enable bool) (bool, bool) {
	var (
		configChanged = false
		configFailed  = false
	)

	uartLine := "enable_uart=0"
	if enable {
		uartLine = "enable_uart=1"
	}
	b.logger.Debugf("Bluetooth parameter configuration - updateBTenableuart: target=%s", uartLine)

	// Detect if the desired line already exists
	found, err := rpiutils.DetectConfigParam(configPath, uartLine, b.logger)
	if err != nil {
		b.logger.Errorf("Bluetooth parameter configuration - DetectConfigParam(%q) error: %v", uartLine, err)
		return false, false
	}
	if found {
		b.logger.Debugf("Bluetooth parameter configuration - found existing %s; no change needed", uartLine)
		return false, false
	}

	// Remove the opposite setting if present
	var removeLine string
	if enable {
		removeLine = "enable_uart=0"
	} else {
		removeLine = "enable_uart=1"
	}
	if removed, err := rpiutils.RemoveConfigParam(configPath, removeLine, b.logger); err != nil {
		b.logger.Errorf("Bluetooth parameter configuration - Failed to remove %s from boot config.txt: %v", removeLine, err)
		configFailed = true
	} else if removed {
		configChanged = true
	}

	if !configFailed {
		// Add the desired setting, only if the removal of the prior parameter succeeded
		b.logger.Infof("Bluetooth parameter configuration - Setting %s in config.txt", uartLine)
		changed, err := rpiutils.UpdateConfigFile(configPath, "enable_uart", "="+map[bool]string{true: "1", false: "0"}[enable], b.logger)
		if err != nil {
			b.logger.Errorf("Bluetooth parameter configuration - Failed to add %s to boot config.txt: %v", uartLine, err)
			configFailed = true
		} else if changed {
			configChanged = true
		}
	}

	return configChanged, configFailed
}

// updateBTminiuart adds or removes dtoverlay=miniuart-bt.
func (b *pinctrlpi5) updateBTminiuart(configPath string, enable bool) (bool, bool) {
	var (
		configChanged bool
		configFailed  bool
	)

	const line = "dtoverlay=miniuart-bt"
	b.logger.Debugf("Bluetooth parameter configuration - updateBTminiuart: dtoverlay=miniuart-bt presence should be %v", enable)

	found, err := rpiutils.DetectConfigParam(configPath, line, b.logger)
	if err != nil {
		b.logger.Errorf("Bluetooth parameter configuration - DetectConfigParam(%q) error: %v", line, err)
		return false, false
	}

	if enable {
		if found {
			b.logger.Debugf("Bluetooth parameter configuration - Found existing %s; no change needed", line)
			return false, false
		}
		b.logger.Infof("Bluetooth parameter configuration - Adding %s to config.txt", line)
		if changed, err := rpiutils.UpdateConfigFile(configPath, line, "", b.logger); err != nil {
			b.logger.Errorf("Bluetooth parameter configuration - Failed to add %s to boot config.txt: %v", line, err)
			configFailed = true
		} else if changed {
			configChanged = true
		}
	} else {
		if !found {
			b.logger.Debugf("Bluetooth parameter configuration - %s not present; no change needed", line)
			return false, false
		}
		b.logger.Infof("Bluetooth parameter configuration - Removing %s from config.txt", line)
		if removed, err := rpiutils.RemoveConfigParam(configPath, line, b.logger); err != nil {
			b.logger.Errorf("Bluetooth parameter configuration - Failed to remove %s from boot config: %v", line, err)
			configFailed = true
		} else if removed {
			configChanged = true
		}
	}

	return configChanged, configFailed
}

// updateBTbaudrate ensures dtparam=krnbt_baudrate is set to the requested value,
// or removed entirely.
func (b *pinctrlpi5) updateBTbaudrate(configPath string, rate int) (bool, bool) {
	var (
		configChanged = false
		configFailed  = false
	)

	baseKey := "dtparam=krnbt_baudrate"
	baudLine := baseKey + "=" + strconv.Itoa(rate)

	if rate == 0 {
		// When 0: remove any dtparam=krnbt_baudrate line(s)
		b.logger.Debugf("Bluetooth parameter configuration - updateBTbaudrate: rate==0; removing any %s entries", baseKey)
		b.logger.Infof("Bluetooth parameter configuration - Removing %s entries from config.txt", baseKey)
		if removed, err := rpiutils.RemoveConfigParam(configPath, baseKey, b.logger); err != nil {
			b.logger.Errorf("Bluetooth parameter configuration - Failed to remove %s from boot config.txt: %v", baseKey, err)
			configFailed = true
		} else if removed {
			configChanged = true
		}
		return configChanged, configFailed
	}

	// Non-zero rate: ensure exact line exists; if different value exists, replace.
	b.logger.Debugf("Bluetooth parameter configuration - updateBTbaudrate: target=%s", baudLine)

	// If the exact desired line already exists, nothing to do.
	foundExact, err := rpiutils.DetectConfigParam(configPath, baudLine, b.logger)
	if err != nil {
		b.logger.Errorf("Bluetooth parameter configuration - DetectConfigParam(%q) error: %v", baudLine, err)
		return false, false
	}
	if foundExact {
		b.logger.Debugf("Bluetooth parameter configuration - Found existing %s; no change needed", baudLine)
		return false, false
	}

	// Remove any existing dtparam=krnbt_baudrate lines (with any value)
	b.logger.Infof("Bluetooth parameter configuration - Removing any existing %s entries", baseKey)
	if removed, err := rpiutils.RemoveConfigParam(configPath, baseKey, b.logger); err != nil {
		b.logger.Errorf("Bluetooth parameter configuration - Failed to remove %s entries from boot config.txt: %v", baseKey, err)
		configFailed = true
	} else if removed {
		configChanged = true
	}

	if !configFailed {
		// Add the desired baudrate line, only if the removal of the prior parameter succeeded
		b.logger.Infof("Bluetooth parameter configuration - Adding %s to config.txt", baudLine)
		if changed, err := rpiutils.UpdateConfigFile(configPath, baseKey, "="+strconv.Itoa(rate), b.logger); err != nil {
			b.logger.Errorf("Bluetooth parameter configuration - Failed to add %s to boot config.txt: %v", baudLine, err)
			configFailed = true
		} else if changed {
			configChanged = true
		}
	}

	return configChanged, configFailed
}

func (b *pinctrlpi5) configureI2C(cfg *rpiutils.Config) {
	b.logger.Debugf("cfg.BoardSettings.I2Cenable=%v", cfg.BoardSettings.I2Cenable)
	// Only enable I2C if turn_i2c_on is true, otherwise do nothing
	if !cfg.BoardSettings.I2Cenable {
		return
	}

	var configChanged, moduleChanged bool
	var err error
	var configFailed, moduleFailed bool

	configChanged, err = b.updateI2CConfig("=on")
	if err != nil {
		b.logger.Errorf("Failed to enable I2C in boot config: %v", err)
		configFailed = true
	}

	moduleChanged, err = b.updateI2CModule(true)
	if err != nil {
		b.logger.Errorf("Failed to enable I2C module: %v", err)
		moduleFailed = true
	}

	if configFailed || moduleFailed {
		b.logger.Errorf("Automatic I2C configuration failed. Please manually enable I2C using 'sudo raspi-config' -> Interfacing Options -> I2C")
		return
	}

	if configChanged || moduleChanged {
		b.logger.Infof("I2C configuration enabled. Initiating automatic reboot...")
		go rpiutils.PerformReboot(b.logger)
	}
}

func (b *pinctrlpi5) updateI2CConfig(desiredValue string) (bool, error) {
	configPath := rpiutils.GetBootConfigPath()
	return rpiutils.UpdateConfigFile(configPath, "dtparam=i2c_arm", desiredValue, b.logger)
}

func (b *pinctrlpi5) updateI2CModule(enable bool) (bool, error) {
	return rpiutils.UpdateModuleFile("/etc/modules", "i2c-dev", enable, b.logger)
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

// checkHardwarePWMOverlayIsConfigured checks if the hardware PWM overlay is enabled in config.txt.
// If not present or commented out returns an error
func checkHardwarePWMOverlayIsConfigured() error {
	configPath := "/boot/firmware/config.txt"
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("couldn't read %s", configPath)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "dtoverlay=pwm-2chan") {
			// dtoverlay=pwm-2chan is uncommented
			return nil
		}
	}

	// If we get here, the overlay is either missing or commented out
	return fmt.Errorf("Hardware PWM overlay is not configured in %s. Some hardware PWM functions may not work. To enable hardware PWM, add 'dtoverlay=pwm-2chan' to your %s file and reboot.", configPath, configPath)
}
