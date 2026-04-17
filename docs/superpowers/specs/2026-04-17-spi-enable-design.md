# SPI Enable Design

**Date:** 2026-04-17

## Goal

Add an `enable_spi` config option to the Raspberry Pi board module that enables SPI0 by writing `dtparam=spi=on` to `/boot/config.txt` (or `/boot/firmware/config.txt`), mirroring exactly what `raspi-config`'s "Enable SPI" option does.

## Config

Add `SPIenable bool` to `BoardSettings` in `utils/config.go`:

```go
type BoardSettings struct {
    I2Cenable bool  `json:"enable_i2c,omitempty"`
    SPIenable bool  `json:"enable_spi,omitempty"`
    // ... BT fields unchanged
}
```

## Implementation

Add `configureSPI()` to both `rpi/board.go` (`piPigpio`) and `pi5/board.go` (`pinctrlpi5`), called from `Reconfigure()` after `configureI2C()`.

The method:
1. Returns early if `SPIenable` is false
2. Calls `rpiutils.UpdateConfigFile` with `dtparam=spi` → `=on`
3. Logs an error and returns nil (no hard failure) if the file update fails
4. Triggers `rpiutils.PerformReboot` in a goroutine if the config changed

No `/etc/modules` changes are needed — unlike I2C (`i2c-dev`), the `spidev` module loads automatically via udev when the SPI device appears.

## Error Handling

Follows the I2C pattern: log errors, instruct the user to run `sudo raspi-config` manually, and return nil rather than failing reconfiguration.

## Out of Scope

- SPI1 / auxiliary bus (`dtoverlay=spi1-Ncs`)
- `/etc/modules` changes
- Changes to analog reader SPI bus handling (already works independently)
