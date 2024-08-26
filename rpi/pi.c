/*
    pi.c: This file is a bridge to setup interrupts for the Raspberry Pi GPIO pins
    using the pigpiod library. It uses a callback function pigpioInterruptCallback
    for interrupt handling, which is exported within rpi.go.
*/
#include <pigpiod_if2.h>

extern void pigpioInterruptCallback(int gpio, int level, uint32_t tick);

// interruptCallback calls through to the go linked interrupt callback.
void interruptCallback(int pi, unsigned gpio, unsigned level, uint32_t tick) {
    if (level == 2) {xwxw
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
    // successful call returns a callback ID that can be used to cancel the callback
    result = callback(pi, gpio, EITHER_EDGE, interruptCallback);
    return result;
}
int setPullUp(int pi, int gpio) {
    int result = set_pull_up_down(pi, gpio, PI_PUD_UP);
    return result;

}

int setPullDown(int pi, int gpio) {
    int result = set_pull_up_down(pi, gpio, PI_PUD_DOWN);
    return result;
}

int setPullNone(int pi, int gpio) {
    int result = set_pull_up_down(pi, gpio, PI_PUD_OFF);
    return result;
}

int teardownInterrupt(int pi, int gpio) {
    int result = callback(pi, gpio, EITHER_EDGE, NULL);
    // Do we need to unset the pullup resistors?
    return result;
}
