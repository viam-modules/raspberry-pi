package rpi

/*
	helper_functions.go: Helper functions used by the Raspberry Pi module.
*/

import rpiutils "viamrpi/utils"

// This is a helper function for digital interrupt reconfiguration. It finds the key in the map
// whose value is the given interrupt, and returns that key and whether we successfully found it.
func findInterruptName(
	interrupt rpiutils.ReconfigurableDigitalInterrupt,
	interrupts map[string]rpiutils.ReconfigurableDigitalInterrupt,
) (string, bool) {
	for key, value := range interrupts {
		if value == interrupt {
			return key, true
		}
	}
	return "", false
}

// This is a very similar helper function, which does the same thing but for broadcom addresses.
func findInterruptBcom(
	interrupt rpiutils.ReconfigurableDigitalInterrupt,
	interruptsHW map[uint]rpiutils.ReconfigurableDigitalInterrupt,
) (uint, bool) {
	for key, value := range interruptsHW {
		if value == interrupt {
			return key, true
		}
	}
	return 0, false
}
