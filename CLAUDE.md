# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Building and Module Creation
- `make module` - Builds the module and creates raspberry-pi-module.tar.gz
- `make build-$(DOCKER_ARCH)` - Builds the binary for specific architecture
- `make setup` - Installs pigpio dependency (requires sudo)

### Testing
- `make test` - Compiles and runs all tests (requires sudo and functioning Raspberry Pi 4)
- Tests are compiled to binaries in bin/ directory and executed with sudo

### Linting
- `make lint` - Runs golangci-lint with project configuration
- `make tool-install` - Installs required linting tools

### Development Commands
- `make update-rdk` - Updates to latest RDK version
- `make clean` - Removes bin/ output directory

## Architecture

### Module Structure
This is a Viam Go module that implements Raspberry Pi board and servo components:

- **main.go**: Module entry point registering all board and servo models
- **rpi/**: Core Raspberry Pi board implementation (models rpi, rpi4, rpi3, rpi2, rpi1, rpi0, rpi0_2)
- **pi5/**: Raspberry Pi 5 specific implementation with different GPIO handling
- **rpi-servo/**: Servo component implementation for Pi 0-4 (not supported on Pi 5)
- **utils/**: Shared utilities including pin mappings, digital interrupts, and errors

### Key Technical Details
- Uses pigpiod daemon for GPIO operations via C bindings (pigpiod_if2.h)
- Supports software PWM with 18 frequencies (8000-10 Hz) at 5μs sample rate
- Implements board API for GPIO, PWM, analog readers, and digital interrupts
- Pi 5 uses different GPIO access method vs older models
- Module builds require canon environment for cross-compilation

### Key Features
- **I2C Auto-Configuration**: Set `enable_i2c: true` in board config to automatically enable I2C interface
- **Cross-Platform I2C Support**: Works on both legacy Pi models (via pigpio) and Pi 5 (via pinctrl)
- **System Integration**: Automatically modifies `/boot/config.txt` and `/etc/modules` for I2C setup
- **Automatic Reboot**: System reboots automatically when I2C configuration changes (3-second delay)

### Dependencies
- pigpio daemon must be installed and running for GPIO functionality
- Requires sudo for test execution due to hardware access
- Built specifically for bullseye/bookworm Debian versions
- I2C configuration requires file system write permissions to `/boot/config.txt` and `/etc/modules`
- Automatic reboot functionality requires sudo permissions for `shutdown -r now`