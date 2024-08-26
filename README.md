# `raspberry-pi`

This module implements the [`"rdk:component:board"` API](https://docs.viam.com/components/board/) and [`"rdk:component:servo"` API](https://docs.viam.com/components/servo/) to integrate the Raspberry Pi 4, 3 and Zero 2 W board or any servos connected to the board into your machine.

This module replaces the `board:pi` and `servo:pi` components in RDK as a step into the modular future of Viam. Furthermore, this module handles the `PI_INIT_FAILED` issue.

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
      // change the model name back to "viam:raspberry-pi:rpi" once this module is public
      "model": "viam-hardware-testing:raspberry-pi:rpi",
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
      "name": "viam-hardware-testing_raspberry-pi",
      "module_id": "viam-hardware-testing:raspberry-pi",
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
      // change the model name back to "viam:raspberry-pi:rpi-servo" once this module is public
      "model": "viam-hardware-testing:raspberry-pi:rpi-servo",
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
      "name": "viam-hardware-testing_raspberry-pi",
      "module_id": "viam-hardware-testing:raspberry-pi",
      "version": "0.0.1"
    }
  ],
}
```

## Building and Using
Module needs to be built from within `canon`. As of August 2024 this module is being built only in `bullseye` and supports `bullseye` and `bookworm` versions of Debian. Simply run `make module` in `canon`. An executable named `raspbery-pi` and a tar named `module.tar.gz` will appear in `bin` folder. 

# Changes from rdk
## Library usage
The module now relies on the **pigpio daemon** to carry out GPIO functionality. The daemon accepts socket and pipe connections over the local network. Although many things can be configured, from DMA allocation mode to socket port to sample rate, we use the default settings, which match with the traditional pigpio library's defaults. More info can be seen here: https://abyz.me.uk/rpi/pigpio/pigpiod.html.

The daemon essentially supports all the same functionality as the traditional library. Instead of using `pigpio.h` C library, it uses the daemon library, which is mostly identical: `pigpiod_if2.h`. The primary difference is how the library is set up. Before, we used `gpioInitialise()` and `gpioTerminate()` to initialize and close the board connection. Now, we must start up the daemon with `sudo pigpiod` and connect to the daemon using the C functions `pigpio_start` and `pigpio_stop`. `pigpio_start` returns an ID that all the daemon library functions take in as the first argument so the daemon knows to use that connection to execute board functionality. Details can be found here: https://abyz.me.uk/rpi/pigpio/pdif2.html

A lot of the work was a simple conversion from the old C library to the new daemon library, which is relatively straightforward, with one notable exception (see callback functions). Some new daemon-specific errors were added to `utils/errors.go`. The errors are mostly related to connecting to the daemon. All other error codes related to GPIO operation remain the same. 

## Struct changes
The `piPigpio` struct must now track the daemon connection ID returned by `pigpio_start`. This `C.int` id is used for most other function calls. This same change is added to the `piPigpioServo` struct, which opens up its own connection to the daemon rather than using the board's. This was just a design choice to preserve simplicity. It is also possible to grab the ID from its dependent board's struct. One thing to note, the pi-servo's dependency is a **soft dependency rather than a hard one.** `Validate` expects there to be a board dependency, but the servo could hypothetically operate independently of its board.

## Structure
The Pi board and the servo are now in module format. The directory structure is as follows:
- `rpi`: Contains all files necessary to define `viam:raspberry-pi:rpi`. Files are organized by functionality.
- `rpi-servo`: Contains all files necessary to define `viam:raspberry-pi:rpi-servo`. Files are organized by functionality
- `utils`: Any utility functions that are either universal to the boards or shared between `rpi` and `rpi-servo`. Included are daemon errors, pin mappings, and digital interrupts
- `testing`: External package exports. Tests the components how an outside package would use the components (w/o any internal functions).

## Digital Interrupts
The old code used two maps to track the digital interrupts: `interrupts` and `interruptsHW`. `interrupts` mapped user-defined interrupt names to interrupt struct, while `interruptsHW` mapped broadcom pin to interrupt struct. The data contained in the maps were the same, just with different map keys.

The implementation tried to preserve interrupt names and interrupts when possible. Essentially, the logic was to preserve interrupts if a name changed and the interrupt stayed the same, or if the name stayed the same but the interrupt changed. It made almost no optimization improvements the way it was handled, as it was a relatively convoluted mess of linear searches `O(n)` through the map values, making sure to update the two interrupt config variables accurately. I realized all of this was very unecessary, as the calls to cancel and start interrupt callbacks (via `pigpio`) were not optimized in any marginal way.
 
Now, we simply have one map, `interrupts`, which **only maps broadcom pin (previously `interruptsHW`) to the interrupt struct**. When reconfiguring the interrupts, all interrupts callbacks are canceled and reinitialized following the new set of interrupts. Essentially, the map is wiped and completely recreated. Although it may seem inefficient (it's not really, since you can only configure so many interrupts), the behavior is identical to the rdk version with less convoluted code. 

Furthermore, the way that daemon callback functions vary from the non-daemon pigpio library. Before, we used `gpioSetAlertFunc` to set up all interruot callbacks. It also used this function to unset callbacks, where a `NULL` function is passed as the function to initialize a callback cancellation. Within the daemon library, a callback is initialized with `callback`, which returns a callback ID. This ID is used to cancel the callback via a separate function `callback_cancel`, which will cancel said callback. There was no ID before, which means we modified the interrupt struct to track this callback ID. All we do now is to wrap the interrupt via a new `rpiStruct`, which holds the old `ReconfigurableDigitalInterrupt` as well as the callback ID. In the `pi.c` library, it takes a callback id to cancel the callbacks now. This is the biggest logical change in this new module.

## Testing
New tests have been written for servo and board. All old tests were preserved. For this module, we implement black box and white box testing. White box uses internal package functions (lowercase functions like newPigpio) and tests the behavior. It also tests any helper functions. Black box testing treats the package as an outside, only using exported functions to test their behavior. This is why we have a new `testing/` folder which holds these tests, which are almost identical to the pacakge tests, without using some private functions. 

All tests require a functioning raspberry pi. 

**Make sure when testing that the testing packages are built as a binary and executed as root (sudo).** Otherwise, some test cases will be skipped without warning (may need verbose flags). Those commands can be seen here:
```bash
CGO_LDFLAGS='-lpigpiod_if2' CGO_ENABLED=1 GOARCH=arm64 CC=aarch64-linux-gnu-gcc go test -c -o ./bin/ raspberry-pi/...
# run test (-test.v for verbose)
sudo ./bin/${test_package}.test
```

## Servo
Servo now uses PWM for more granular control. It essentially performs the same behavior as before, but uses PWM functions to mimic the servo functions within the `pigpio` library. It explains how to do it here: https://abyz.me.uk/rpi/pigpio/pdif2.html#set_servo_pulsewidth.

Before, we only had servo control at 50Hz. We can now control at more granular frequencies (following the chart in the link above) using PWM, allowing the user to enter a `frequency_hz` parameter in order to control the servo refresh rate. As a consequence, some functions were reorganized to carry this functionality out.

## Analog Readers and SPI
The analog readers used SPI in order to transfer information. The SPI previously used pigpio defined SPI functions. We wanted to use our [genericlinux](https://github.com/viamrobotics/rdk/tree/main/components/board/genericlinux) implementation, which the library now uses. We simply set up a `NewSPIBus` and everything works well. There are chip select mappings (within `rpi/analog_readers.go`) that map physical pins to chip select pins. Otherwise, behavior is the same.

## Starting and Stopping `pigpiod`
The daemon is automatically started in the module on init and shut down on Close(). There are some tricky consequences to this:
- the daemon has a startup period. I've noticed on a clean board that it starts within 1-50ms. Trying to use any C functions before then will result in connection errors
- The daemon stops almost immidiately when Close() is called. If the daemon is reading data (such as: GPIO) the module may encounter the following message ``` notify thread from pi 1 broke with read error 0 ``` This was only objserved during testing when the daemon was stopped and started immidietly. 

## Other Considerations
- I2C has been removed. It wasn't used for anything.
