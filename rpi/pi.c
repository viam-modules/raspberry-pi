//go:build !no_pigpio
#include <pigpiod_if2.h>

extern void pigpioInterruptCallback(int gpio, int level, uint32_t tick);

// interruptCallback calls through to the go linked interrupt callback.
void interruptCallback(int pi, unsigned gpio, unsigned level, uint32_t tick) {
    if (level == 2) {
        // watchdog
        return;
    }
    pigpioInterruptCallback(gpio, level, tick);
}

int setupInterrupt(int pi, int gpio) {
    int result = set_mode(pi, gpio, PI_INPUT);
    if (result != 0) {
        return result;
    }
    result = set_pull_up_down(pi, gpio, PI_PUD_UP); // should this be configurable?
    if (result != 0) {
        return result;
    }
    result = callback(pi, gpio, EITHER_EDGE, interruptCallback);
    return result;
}

int teardownInterrupt(int pi, int gpio) {
    int result = callback(pi, gpio, EITHER_EDGE, NULL);
    // Do we need to unset the pullup resistors?
    return result;
}

int custom_pigpio_start() {
    return pigpio_start(NULL, NULL);
}