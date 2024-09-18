# [`raspberry-pi` module](https://app.viam.com/module/viam/raspberry-pi)

This module implements the [`rdk:component:board` API](https://docs.viam.com/components/board/) and [`rdk:component:servo` API](https://docs.viam.com/components/servo/)

Two models are provided in this module:
* `viam:raspberry-pi:rpi` - Configure a Raspberry Pi 4, 3 and Zero 2 W,  board to access GPIO functionality: input, output, PWM, power, serial interfaces, etc.
* `viam:raspberry-pi:rpi-servo` - Configure a servo controlled by the GPIO pins on the board.

## Configure your `raspberry-pi` board

Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com), searching for `raspberry-pi`

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
### Additional Info for Dev
We use our [genericlinux implementation](https://github.com/viamrobotics/rdk/tree/main/components/board/genericlinux) for SPI and I2C. 

## Configure your pi servo
Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com), searching for `rpi-servo`

Fill in the attributes as applicable to your servo, according to the example below. The configuration is the same as the [servo docs](hhttps://docs.viam.com/components/servo/pi/).

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

### `frequency_hz`
The one new addition is the ability to change the servo frequency (`frequency: hz`). You should look at your part's documentation to determine the optimal operating frequency and operating rotation range.
Otherwise, the config is the same as the [servo docs](https://docs.viam.com/components/servo/pi/). In the module, the servo now uses PWM for more granular control. It essentially performs the same behavior as before, but uses PWM functions to mimic the servo functions within the pigpio library. It explains how to do it here: https://abyz.me.uk/rpi/pigpio/pdif2.html#set_servo_pulsewidth.

Before, we only had servo control at 50Hz. We can now control at more granular frequencies (following the chart in the link above) using PWM, allowing the user to enter a `frequency_hz` parameter in order to control the servo refresh rate.

## Local development

### Building 
Module needs to be built from within `canon`. As of August 2024 this module is being built only in `bullseye` and supports `bullseye` and `bookworm` versions of Debian. 
`make module` will create raspberry-pi-module.tar.gz.
```bash
canon 
make module
```
Then copy the tar.gz over to your pi 
```bash 
scp /path-to/raspberry-pi-module.tar.gz your_rpi@pi.local:~
```
Untar the tar.gz file and execute `run.sh`

### Linting 
Linting also needs to be done from within `canon` 
```bash
canon 
make lint
```
### Testing 
> [!NOTE]
>All tests require a functioning raspberry pi4!

Run the following in a pi
```bash
make test
```
This will create binaries for each test file in /bin and run them.

## For Devs
### Module Structure
The directory structure is as follows:
- `rpi`: Contains all files necessary to define `viam:raspberry-pi:rpi`. Files are organized by functionality.
- `rpi-servo`: Contains all files necessary to define `viam:raspberry-pi:rpi-servo`. Files are organized by functionality
- `utils`: Any utility functions that are either universal to the boards or shared between `rpi` and `rpi-servo`. Included are daemon errors, pin mappings, and digital interrupts
- `testing`: External package exports. Tests the components how an outside package would use the components (w/o any internal functions).

### pigpiod
The module relies on the pigpio daemon to carry out GPIO functionality. The daemon accepts socket and pipe connections over the local network. Although many things can be configured, from DMA allocation mode to socket port to sample rate, we use the default settings, which match with the traditional pigpio library's defaults. More info can be seen here: https://abyz.me.uk/rpi/pigpio/pigpiod.html.

The daemon essentially supports all the same functionality as the traditional library. Instead of using pigpio.h C library, it uses the daemon library, which is mostly identical: pigpiod_if2.h. Details can be found here: https://abyz.me.uk/rpi/pigpio/pdif2.html
