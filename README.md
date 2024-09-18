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

## Building and Using Locally
Module needs to be built from within `canon`. As of August 2024 this module is being built only in `bullseye` and supports `bullseye` and `bookworm` versions of Debian. Simply run `make build` in `canon`. An executable named `raspberry-pi` will appear in `bin` folder. 

## Structure
The Pi board and the servo are now in module format. The directory structure is as follows:
- `rpi`: Contains all files necessary to define `viam:raspberry-pi:rpi`. Files are organized by functionality.
- `rpi-servo`: Contains all files necessary to define `viam:raspberry-pi:rpi-servo`. Files are organized by functionality
- `utils`: Any utility functions that are either universal to the boards or shared between `rpi` and `rpi-servo`. Included are daemon errors, pin mappings, and digital interrupts
- `testing`: External package exports. Tests the components how an outside package would use the components (w/o any internal functions).

## Testing Locally
All tests require a functioning raspberry pi4!

**Make sure when testing that the testing packages are built as a binary and executed as root (sudo).** Otherwise, some test cases will be skipped without warning (may need verbose flags). Those commands can be seen here:
```bash
CGO_LDFLAGS='-lpigpiod_if2' CGO_ENABLED=1 GOARCH=arm64 CC=aarch64-linux-gnu-gcc go test -c -o ./bin/ raspberry-pi/...
# run test (-test.v for verbose)
sudo ./bin/${test_package}.test
```

## Starting and Stopping `pigpiod`
The daemon is automatically started in the module on init and shut down on Close(). There are some tricky consequences to this:
- the daemon has a startup period. On a clean board that it starts within 1-50ms. Trying to use any C functions before then will result in connection errors
- The daemon stops almost immidiately when Close() is called. If the daemon is reading data (such as: GPIO) the module may encounter the following message ``` notify thread from pi 1 broke with read error 0 ``` This was only objserved during testing when the daemon was stopping and starting immediately. 

TODO: update this readme to follow module template before publishing
