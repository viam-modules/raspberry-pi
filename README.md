# [`raspberry-pi` module](https://app.viam.com/module/viam/raspberry-pi)

This module implements the [`rdk:component:board` API](https://docs.viam.com/components/board/#api) and [`rdk:component:servo` API](https://docs.viam.com/components/servo/#api)

This module provides the following models to access GPIO functionality (input, output, PWM, power, serial interfaces, etc.):

* `viam:raspberry-pi:rpi5` - Configure a Raspberry Pi 5 board 
* `viam:raspberry-pi:rpi4` - Configure a Raspberry Pi 4 board
* `viam:raspberry-pi:rpi3` - Configure a Raspberry Pi 3 board
* `viam:raspberry-pi:rpi2` - Configure a Raspberry Pi 2 board
* `viam:raspberry-pi:rpi1` - Configure a Raspberry Pi 1 board
* `viam:raspberry-pi:rpi0` - Configure a Raspberry Pi 0 board
* `viam:raspberry-pi:rpi0_2` - Configure a Raspberry Pi 0 2 W board

This module also provides a servo model:

* `viam:raspberry-pi:rpi-servo` - Configure a servo controlled by the GPIO pins on the board. Note this model is not supported on the rpi5 board.

All of the Pis above are supported as [board components](https://docs.viam.com/operate/reference/components/board/), but some older models are not capable of running `viam-server`--see [Set up a computer or SBC](https://docs.viam.com/operate/get-started/setup/) for `viam-server` system requirements.

## Requirements

Follow the [setup guide](https://docs.viam.com/installation/prepare/rpi-setup/) to prepare your Pi for running `viam-server` before configuring this board.

Navigate to the **CONFIGURE** tab of your machine's page in the [Viam app](https://app.viam.com), searching for `raspberry-pi`.

## Configure your `raspberry-pi` board

You can copy the following optional attributes to your json if you want to configure `pins`, `analogs`, and `board_settings`. These are not required to use the Raspberry Pi.

```json
{
  "pins": [{ }],
  "analogs": [{ } ],
  "board_settings": {
    "enable_i2c": true
  }
}
```

### `pins`
Pins can be configured as GPIO pins and interrupts. [Interrupts](https://en.wikipedia.org/wiki/Interrupt) are a method of signaling precise state changes. Configuring digital interrupts to monitor GPIO pins on your board is useful when your application needs to know precisely when there is a change in GPIO value between high and low.
Example JSON Configuration:

```json
{
  "pins": [
    {
      "name": "your-gpio-1",
      "pin": "13",
      "type": "gpio"
    },
    {
      "name": "your-gpio-2",
      "pin": "14",
      "pull": "down"
    },
    {
      "name": "your-interrupt-1",
      "pin": "15",
      "type": "interrupt"
    },
    {
      "name": "your-interrupt-2",
      "pin": "16",
      "type": "interrupt",
      "pull": "down"
    }
  ]
}
```

The following attributes are available for `pins`:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
|`pin`| string | **Required** | The pin number of the board's GPIO pin that you wish to configure the digital interrupt for. |
|`name` | string | Optional | Your name for the digital interrupt. |
|`type`| string | Optional | Whether the pin should be an `interrupt` or `gpio` pin. Default: `"gpio"` |
|`pull`| string | Optional | Define whether the pins should be pull up or pull down. Omitting this uses your Pi's default configuration |
|`debounce_ms`| string | Optional | define a signal debounce for your interrupts to help prevent false triggers. </li> </ul> |

* When an interrupt configured on your board processes a change in the state of the GPIO pin it is configured to monitor, it ticks to record the state change. You can stream these ticks with the board API's [`StreamTicks()`](https://docs.viam.com/components/board/#streamticks), or get the current value of the digital interrupt with Value().
* Calling [`GetGPIO()`](https://docs.viam.com/components/board/#getgpio) on a GPIO pin, which you can do without configuring interrupts, is useful when you want to know a pin's value at specific points in your program, but is less precise and convenient than using an interrupt.

### `analogs`

An [analog-to-digital converter](https://www.electronics-tutorials.ws/combination/analogue-to-digital-converter.html) (ADC) takes a continuous voltage input (analog signal) and converts it to an discrete integer output (digital signal).

ADCs are useful when building a robot, as they enable your board to read the analog signal output by most types of [sensors](https://docs.viam.com/components/sensor/) and other hardware components.

To integrate an ADC into your machine, you must first physically connect the pins on your ADC to your board. The Pi 5 board does not currently support the use of analogs.

Then, integrate `analogs` into the `attributes` of your board by adding the following to your board's JSON configuration:

```json
"analogs": [
  {
    "name": "current",
    "channel": "1",
    "spi_bus": "1",
    "chip_select": "0",
    "average_over_ms": 10,
    "samples_per_sec": 1
  },
  {
    "name": "pressure",
    "channel": "0",
    "spi_bus": "1",
    "chip_select": "0"
  }
]
```

The following attributes are available for `analogs`:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `name` | string | **Required** | Your name for the analog reader. |
| `channel` | string | **Required** | The pin number of the ADC's connection pin, wired to the board. This should be labeled as the physical index of the pin on the ADC. |
| `chip_select` | string | **Required** | The chip select index of the board's connection pin, wired to the ADC. |
| `spi_bus` | string | **Required** | The index of the SPI bus connecting the ADC and board. |
| `average_over_ms` | int | Optional | Duration in milliseconds over which the rolling average of the analog input should be taken. |
| `samples_per_sec` | int | Optional | Sampling rate of the analog input in samples per second. |

### `board_settings`

The `board_settings` section allows you to configure board-level settings.

#### `enable_i2c`

The I2C interface on Raspberry Pi is disabled by default. When you set `enable_i2c` to `true`, the module will automatically configure your Raspberry Pi to enable I2C communication.

```json
{
  "board_settings": {
    "enable_i2c": true
  }
}
```

When I2C is enabled, the module will:

1. Add `dtparam=i2c_arm=on` to `/boot/config.txt` (or `/boot/firmware/config.txt` on newer systems).
2. Add `i2c-dev` to `/etc/modules` to ensure the I2C device interface is available.
3. Log the configuration changes for your reference.
4. **Automatically reboot the system** if changes were made.

**Important Notes:**
- The system will automatically reboot when I2C configuration changes are made.
- If I2C is already enabled, no reboot will occur.

The following attributes are available for I2C configuration:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `board_settings` | object | Optional | Board-level configuration settings |
| `board_settings.enable_i2c` | boolean | Optional | Enable I2C interface on the Raspberry Pi. Default: `false` |

## Configure your pi servo

Navigate to the **CONFIGURE** tab of your machine's page in the [Viam app](https://app.viam.com), searching for `rpi-servo`

Fill in the attributes as applicable to your servo, according to the example below.

```json
{
  "pin": "11",
  "board": "board-`",
}
```

### Servo attributes

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

* `rpi`: Contains all files necessary to define `viam:raspberry-pi:rpi`. Files are organized by functionality.
* `rpi-servo`: Contains all files necessary to define `viam:raspberry-pi:rpi-servo`. Files are organized by functionality
* `utils`: Any utility functions that are either universal to the boards or shared between `rpi` and `rpi-servo`. Included are daemon errors, pin mappings, and digital interrupts
* `testing`: External package exports. Tests the components how an outside package would use the components (w/o any internal functions).

### pigpiod

The module relies on the pigpio daemon to carry out GPIO functionality. The daemon accepts socket and pipe connections over the local network. Although many things can be configured, from DMA allocation mode to socket port to sample rate, we use the default settings, which match with the traditional pigpio library's defaults. More info can be seen here: <https://abyz.me.uk/rpi/pigpio/pigpiod.html>.

The daemon essentially supports all the same functionality as the traditional library. Instead of using pigpio.h C library, it uses the daemon library, which is mostly identical: pigpiod_if2.h. Details can be found here: <https://abyz.me.uk/rpi/pigpio/pdif2.html>

### Next steps

To test your board or servo, click on the [**Test** panel](https://docs.viam.com/fleet/control) on your component's configuration page or on the **CONTROL** page.

If you want to see how you can make an LED blink with your Raspberry Pi, see [Make an LED Blink With Buttons And With Code](https://docs.viam.com/tutorials/get-started/blink-an-led/).
