//go:build !no_pigpio
#pragma once

// interruptCallback calls through to the go linked interrupt callback.
int setupInterrupt(int pi, int gpio);
int teardownInterrupt(int pi, int gpio);
int custom_pigpio_start();