# [`raspberry-pi` module](https://app.viam.com/module/viam/raspberry-pi)

This module implements the [`rdk:component:board` API](https://docs.viam.com/components/board/#api) and [`rdk:component:servo` API](https://docs.viam.com/components/servo/#api)

Two models are provided in this module:
* `viam:raspberry-pi:rpi` - Configure a Raspberry Pi 4, 3 and Zero 2 W,  board to access GPIO functionality: input, output, PWM, power, serial interfaces, etc.
* `viam:raspberry-pi:rpi-servo` - Configure a servo controlled by the GPIO pins on the board.

## Requirements

Follow the [setup guide](https://docs.viam.com/installation/prepare/rpi-setup/) to prepare your Pi for running `viam-server` before configuring this board.

## Configure your `raspberry-pi` board

Navigate to the **CONFIGURE** tab of your machine's page in the [Viam app](https://app.viam.com), searching for `raspberry-pi`.

Fill in the attributes as applicable to your board:

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

### Attributes

The following attributes are available for `viam:raspberry-pi:rpi` board:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `analogs` | object | Optional | Attributes of any pins that can be used as analog-to-digital converter (ADC) inputs. See [configuration info](#analogs). |
| `digital_interrupts` | object | Optional | Any digital interrupts's pin number" and name. See [configuration info](#digital_interrupts). |

#### `analogs`

An [analog-to-digital converter](https://www.electronics-tutorials.ws/combination/analogue-to-digital-converter.html) (ADC) takes a continuous voltage input (analog signal) and converts it to an discrete integer output (digital signal).

ADCs are useful when building a robot, as they enable your board to read the analog signal output by most types of [sensors](https://docs.viam.com/components/sensor/) and other hardware components.

To integrate an ADC into your machine, you must first physically connect the pins on your ADC to your board.

Then, integrate `analogs` into the `attributes` of your board by following the **Config Builder** instructions or by adding the following to your board’s JSON configuration:

```json
// "attributes": { ... ,
"analogs": [
  {
    "name": "<your-analog-reader-name>",
    "pin": "<pin-number-on-adc>",
    "spi_bus": "<your-spi-bus-index>",
    "chip_select": "<chip-select-index>",
    "average_over_ms": <int>,
    "samples_per_sec": <int>
  }
]
```

The following properties are available for `analogs`:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `name` | string | **Required** | Your name for the analog reader. |
| `pin` | string | **Required** | The pin number of the ADC's connection pin, wired to the board. This should be labeled as the physical index of the pin on the ADC.
| `chip_select` | string | **Required** | The chip select index of the board's connection pin, wired to the ADC. |
| `spi_bus` | string | **Required** | The index of the SPI bus connecting the ADC and board. |
| `average_over_ms` | int | Optional | Duration in milliseconds over which the rolling average of the analog input should be taken. |
| `samples_per_sec` | int | Optional | Sampling rate of the analog input in samples per second. |

Example:

```json
{
  "components": [
    {
      "model": "pi",
      "name": "your-board",
      "type": "board",
      "attributes": {
        "analogs": [
          {
            "name": "current",
            "pin": "1",
            "spi_bus": "1",
            "chip_select": "0"
          },
          {
            "name": "pressure",
            "pin": "0",
            "spi_bus": "1",
            "chip_select": "0"
          }
        ]
      }
    }
  ]
}
```

#### `digital_interrupts`

[Interrupts](https://en.wikipedia.org/wiki/Interrupt) are a method of signaling precise state changes. Configuring digital interrupts to monitor GPIO pins on your board is useful when your application needs to know precisely when there is a change in GPIO value between high and low.

- When an interrupt configured on your board processes a change in the state of the GPIO pin it is configured to monitor, it ticks to record the state change. You can stream these ticks with the board API’s [`StreamTicks()`](https://docs.viam.com/components/board/#streamticks), or get the current value of the digital interrupt with Value().
- Calling [`GetGPIO()`](https://docs.viam.com/components/board/#getgpio) on a GPIO pin, which you can do without configuring interrupts, is useful when you want to know a pin’s value at specific points in your program, but is less precise and convenient than using an interrupt.

Integrate `digital_interrupts` into your machine in the `attributes` of your board by following the **Config Builder** instructions, or by adding the following to your board’s JSON configuration:

```json
// "attributes": { ... ,
"digital_interrupts": [
  {
    "name": "<your-digital-interrupt-name>",
    "pin": "<pin-number>"
  }
]
```

The following properties are available for `digital_interrupts`:
| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
|`name` | string | **Required** | Your name for the digital interrupt. |
|`pin`| string | **Required** | The pin number of the board's GPIO pin that you wish to configure the digital interrupt for. |
|`type`| string | Optional | <ul><li>`basic`: Recommended. Tracks interrupt count. </li> <li>`servo`: For interrupts configured for a pin controlling a [servo](https://docs.viam.com/components/servo/). Tracks pulse width value. </li></ul> |

Example:

```json
{
  "components": [
    {
      "model": "pi",
      "name": "your-board",
      "type": "board",
      "attributes": {
        "digital_interrupts": [
          {
            "name": "your-interrupt-1",
            "pin": "15"
          },
          {
            "name": "your-interrupt-2",
            "pin": "16"
          }
        ]
      }
    }
  ]
}
```

### Additional Info for Dev
This module uses Viam's [genericlinux implementation](https://github.com/viamrobotics/rdk/tree/main/components/board/genericlinux) for SPI and I2C. 

## Configure your pi servo
Navigate to the **CONFIGURE** tab of your machine's page in the [Viam app](https://app.viam.com), searching for `rpi-servo`

Fill in the attributes as applicable to your servo, according to the example below.

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

### Attributes


The following attributes are available for `viam:raspberry-pi:rpi-servo` servos:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `pin` | string | **Required** | The pin number of the pin the servo's control wire is wired to on the board. |
| `board` | string | **Required** | `name` of the board the servo is wired to. |
| `min` | float | Optional | Sets a software limit on the minimum angle in degrees your servo can rotate to. <br> Default = `0.0` <br> Range = [`0.0`, `180.0`] |
| `max` | float | Optional | Sets a software limit on the maximum angle in degrees your servo can rotate to. <br> Default = `180.0` <br> Range = [`0.0`, `180.0`] |
| `starting_position_degs` | float | Optional | Starting position of the servo in degrees. <br> Default = `0.0` <br> Range = [`0.0`, `180.0`] |
| `hold_position` | boolean | Optional | If `false`, power down a servo if it has tried and failed to go to a position for a duration of 500 milliseconds. <br> Default = `true` |
| `max_rotation_deg` | int | Optional | The maximum angle that you know your servo can possibly rotate to, according to its hardware. Refer to your servo's data sheet for clarification. Must be greater than or equal to the value you set for `max`. <br> Default = `180` |
| `frequency_hz` | int | Optional | Servo refresh rate control value. Use this value to control the servo at more granular frequencies. Refer to your servo's data sheet for optimal operating frequency and operating rotation range. Default: `50` |

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
Now you can use it as a [local module](https://docs.viam.com/how-tos/create-module/#test-your-module-locally)!

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

### Next steps

To test your board or servo, click on the [**Test** panel](https://docs.viam.com/fleet/control) on your component's configuration page or on the **CONTROL** page.

If you want to see how you can make an LED blink with your Raspberry Pi, see [Make an LED Blink With Buttons And With Code](https://docs.viam.com/tutorials/get-started/blink-an-led/).
