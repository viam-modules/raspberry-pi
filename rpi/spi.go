package rpi

/*
	This driver implements SPI functionality for the Raspberry Pi using the pigpio daemon.
	This will likely soon be deprecated to use genericlinux implementation instead.
*/

// #include <stdlib.h>
// #include <pigpiod_if2.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"
	"sync"
	rpiutils "viamrpi/utils"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board/genericlinux/buses"
)

type piPigpioSPI struct {
	pi           *piPigpio
	mu           sync.Mutex
	busSelect    string
	openHandle   *piPigpioSPIHandle
	nativeCSSeen bool
	gpioCSSeen   bool
}

type piPigpioSPIHandle struct {
	bus      *piPigpioSPI
	isClosed bool
}

func (s *piPigpioSPIHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	if s.isClosed {
		return nil, errors.New("can't use Xfer() on an already closed SPIHandle")
	}

	var spiFlags uint
	var gpioCS bool
	var nativeCS C.uint

	if s.bus.busSelect == "1" {
		spiFlags |= 0x100 // Sets AUX SPI bus bit
		if mode == 1 || mode == 3 {
			return nil, errors.New("AUX SPI Bus doesn't support Mode 1 or Mode 3")
		}
		if chipSelect == "11" || chipSelect == "12" || chipSelect == "36" {
			s.bus.nativeCSSeen = true
			if chipSelect == "11" {
				nativeCS = 1
			} else if chipSelect == "36" {
				nativeCS = 2
			}
		} else {
			s.bus.gpioCSSeen = true
			gpioCS = true
		}
	} else {
		if chipSelect == "24" || chipSelect == "26" {
			s.bus.nativeCSSeen = true
			if chipSelect == "26" {
				nativeCS = 1
			}
		} else {
			s.bus.gpioCSSeen = true
			gpioCS = true
		}
	}

	// Libpigpio will always enable the native CS output on 24 & 26 (or 11, 12, & 36 for aux SPI)
	// Thus you don't have anything using those pins even when we're directly controlling another (extended/gpio) CS line
	// Use only the native CS pins OR don't use them at all
	if s.bus.nativeCSSeen && s.bus.gpioCSSeen {
		return nil, errors.New("pi SPI cannot use both native CS pins and extended/gpio CS pins at the same time")
	}

	// Bitfields for mode
	// Mode POL PHA
	// 0    0   0
	// 1    0   1
	// 2    1   0
	// 3    1   1
	spiFlags |= mode

	count := len(tx)
	rx := make([]byte, count)
	rxPtr := C.CBytes(rx)
	defer C.free(rxPtr)
	txPtr := C.CBytes(tx)
	defer C.free(txPtr)

	handle := C.spi_open(C.int(s.bus.pi.piID), nativeCS, (C.uint)(baud), (C.uint)(spiFlags))

	if handle < 0 {
		errMsg := fmt.Sprintf("error opening SPI Bus %s, flags were %X", s.bus.busSelect, spiFlags)
		return nil, rpiutils.ConvertErrorCodeToMessage(int(handle), errMsg)
	}
	defer C.spi_close(C.int(s.bus.pi.piID), (C.uint)(handle))

	if gpioCS {
		// We're going to directly control chip select (not using CE0/CE1/CE2 from SPI controller.)
		// This allows us to use a large number of chips on a single bus.
		// Per "seen" checks above, cannot be mixed with the native CE0/CE1/CE2
		chipPin, err := s.bus.pi.GPIOPinByName(chipSelect)
		if err != nil {
			return nil, err
		}
		err = chipPin.Set(ctx, false, nil)
		if err != nil {
			return nil, err
		}
	}

	ret := C.spi_xfer(C.int(s.bus.pi.piID), (C.uint)(handle), (*C.char)(txPtr), (*C.char)(rxPtr), (C.uint)(count))

	if gpioCS {
		chipPin, err := s.bus.pi.GPIOPinByName(chipSelect)
		if err != nil {
			return nil, err
		}
		err = chipPin.Set(ctx, true, nil)
		if err != nil {
			return nil, err
		}
	}

	if int(ret) != count {
		return nil, errors.Errorf("error with spiXfer: Wanted %d bytes, got %d bytes", count, ret)
	}

	return C.GoBytes(rxPtr, (C.int)(count)), nil
}

func (s *piPigpioSPI) OpenHandle() (buses.SPIHandle, error) {
	s.mu.Lock()
	s.openHandle = &piPigpioSPIHandle{bus: s, isClosed: false}
	return s.openHandle, nil
}

func (s *piPigpioSPI) Close(ctx context.Context) error {
	return nil
}

func (s *piPigpioSPIHandle) Close() error {
	s.isClosed = true
	s.bus.mu.Unlock()
	return nil
}
