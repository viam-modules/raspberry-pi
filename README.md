# [`raspberry-pi` module](https://app.viam.com/module/viam/raspberry-pi)

This module implements the [`rdk:component:board` API](https://docs.viam.com/components/board/) and [`rdk:component:servo` API](https://docs.viam.com/components/servo/)

Two models are provided in this module:
* `viam:raspberry-pi:rpi` - Configure a Raspberry Pi 4, 3 and Zero 2 W,  board to access GPIO functionality: input, output, PWM, power, serial interfaces, etc.
* `viam:raspberry-pi:rpi-servo` - Configure a servo controlled by the GPIO pins on the board.

## Requirements

Follow the [setup guide](https://docs.viam.com/installation/prepare/rpi-setup/) to prepare your Pi for running `viam-server` before configuring this board.

## Configure your `raspberry-pi` board

Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com), searching for `raspberry-pi`.

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
| `digital_interrupts` | object | Optional | Any digital interrupts's {{< glossary_tooltip term_id="pin-number" text="pin number" >}} and name. See [configuration info](#digital_interrupts). |

### Example configuration

```json
{
  <INSERT SAMPLE CONFIGURATION(S)>
}
```

### Additional Info for Dev
This module uses Viam's [genericlinux implementation](https://github.com/viamrobotics/rdk/tree/main/components/board/genericlinux) for SPI and I2C. 

## Configure your pi servo
Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com), searching for `rpi-servo`

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

The following attributes are available for `<INSERT MODEL TRIPLET>` <INSERT API NAME>s:

| Name    | Type   | Required?    | Description |
| ------- | ------ | ------------ | ----------- |
| `todo1` | string | **Required** | TODO        |
| `todo2` | string | Optional     | TODO        |

### Example configuration

```json
{
  <INSERT SAMPLE CONFIGURATION(S)>
}
```

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
Now you can use it as a [local module](https://docs.viam.com/tutorials/configure/pet-photographer/#add-as-a-local-module)!

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
