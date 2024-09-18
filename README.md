# `raspberry-pi`

This module implements the [`"rdk:component:board"` API](https://docs.viam.com/components/board/) and [`"rdk:component:servo"` API](https://docs.viam.com/components/servo/) to integrate the Raspberry Pi 4, 3 and Zero 2 W board or any servos connected to the board into your machine.

Two models are provided:
* `viam:raspberry-pi:rpi` - Configure a Raspberry Pi 4, 3 and Zero 2 W,  board to access GPIO functionality: input, output, PWM, power, serial interfaces, etc.
* `viam:raspberry-pi:rpi-servo` - Configure a servo controlled by the GPIO pins on the board.

## Configure your board

Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com), searching for `raspberry-pi` and selecting one of the above models.

Fill in the attributes as applicable to your board, according to the example below. The configuration is the same as the [board docs](https://docs.viam.com/components/board/pi/).

```json
{
  {
  "components": [
    {
      "name": "<your-pi-board-name>",
      "model": "viam:raspberry-pi:rpi",
      "type": "board",
      "namespace": "rdk",
      "attributes": {
        "analogs": [
          {
            "name": "<your-analog-reader-name>",
            "pin": "<pin-number-on-adc>",
            "spi_bus": "<your-spi-bus-index>",
            "chip_select": "<chip-select-index>",
            "average_over_ms": <int>,
            "samples_per_sec": <int>
          }
        ],
        "digital_interrupts": [
          {
            "name": "<your-digital-interrupt-name>",
            "pin": "<pin-number>"
          }
        ]
      },
    }
  ]
  "modules": [
    {
      "type": "registry",
      "name": "viam_raspberry-pi",
      "module_id": "viam:raspberry-pi",
      "version": "0.0.1"
    }
  ],
}
```
### Analog Readers and SPI
The analog readers used SPI in order to transfer information. The SPI previously used pigpio defined SPI functions. We use our [genericlinux implementation](https://github.com/viamrobotics/rdk/tree/main/components/board/genericlinux) We simply set up a NewSPIBus and everything works well. There are chip select mappings (within rpi/analog_readers.go) that map physical pins to chip select pins. Otherwise, behavior is the same.

## Configure your pi servo
Similarly for the servo. The one new addition is the ability to change the servo frequency (`frequency: hz`). You should look at your part's documentation to determine the optimal operating frequency and operating rotation range.
Otherwise, the config is the same as the [servo docs](https://docs.viam.com/components/servo/pi/).
```json
{
  "components": [
    {
      "name": "<your-servo-name>",
      "model": "viam:raspberry-pi:rpi-servo",
      "type": "servo",
      "namespace": "rdk",
      "attributes": {
        "pin": "<your-pin-number>",
        "board": "<your-board-name>",
        "min": <float>,
        "max": <float>,
        "starting_position_deg": <float>,
        "hold_position": <int>,
        "max_rotation_deg": <int>,
        "frequency_hz": <int>
      }
    }
  ],
  "modules": [
    {
      "type": "registry",
      "name": "viam_raspberry-pi",
      "module_id": "viam:raspberry-pi",
      "version": "0.0.1"
    }
  ],
}
```
### Servo frequency
Servo now uses PWM for more granular control. It essentially performs the same behavior as before, but uses PWM functions to mimic the servo functions within the pigpio library. It explains how to do it here: https://abyz.me.uk/rpi/pigpio/pdif2.html#set_servo_pulsewidth.

Before, we only had servo control at 50Hz. We can now control at more granular frequencies (following the chart in the link above) using PWM, allowing the user to enter a `frequency_hz` parameter in order to control the servo refresh rate.

## Building and Using Locally
Module needs to be built from within `canon`. As of August 2024 this module is being built only in `bullseye` and supports `bullseye` and `bookworm` versions of Debian. Simply run `make build` in `canon`. An executable named `raspberry-pi` will appear in `bin` folder. 
To locally use the module, run `make module` in `canon`, copy over the `raspberry-pi-module.tar.gz` using the command below 
```bash 
scp /path-to/raspberry-pi-module.tar.gz your_rpi@pi.local:~
```
Untar the tar.gz file and execute `run.sh`

## Testing Locally
All tests require a functioning raspberry pi4!
To test, run `make test` in `canon`. This will create binaries for each test file in /bin and run them. 

## Development

### Structure
The directory structure is as follows:
- `rpi`: Contains all files necessary to define `viam:raspberry-pi:rpi`. Files are organized by functionality.
- `rpi-servo`: Contains all files necessary to define `viam:raspberry-pi:rpi-servo`. Files are organized by functionality
- `utils`: Any utility functions that are either universal to the boards or shared between `rpi` and `rpi-servo`. Included are daemon errors, pin mappings, and digital interrupts
- `testing`: External package exports. Tests the components how an outside package would use the components (w/o any internal functions).

The module relies on the pigpio daemon to carry out GPIO functionality. The daemon accepts socket and pipe connections over the local network. Although many things can be configured, from DMA allocation mode to socket port to sample rate, we use the default settings, which match with the traditional pigpio library's defaults. More info can be seen here: https://abyz.me.uk/rpi/pigpio/pigpiod.html.

The daemon essentially supports all the same functionality as the traditional library. Instead of using pigpio.h C library, it uses the daemon library, which is mostly identical: pigpiod_if2.h. Details can be found here: https://abyz.me.uk/rpi/pigpio/pdif2.html
