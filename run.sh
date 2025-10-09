#!/bin/sh

# Pigpio client libraries
DEBIAN_FRONTEND=noninteractive apt install -qqy libpigpiod-if2-1 2>&1

# RPI5 doesn't need pigpiod
WHATPI=$(awk '{print $3}' /proc/device-tree/model)
if [ "$WHATPI" = "5" ]; then
    exec ./bin/raspberry-pi-arm64 "$@" "pi5-detected"
fi

ARCH=$(uname -m)

# Check if pigpiod is already running.
# NOTE: it may be running as a service or locally started process. No attempt is made to control it.
if pgrep -x "pigpiod" > /dev/null; then
    echo "pigpiod is already running, not explicitly starting it."
else
    echo "pigpiod is not running, starting..."

    if [ "$ARCH" = "aarch64" ]; then
        # 64-bit ARM architecture
        ./bin/pigpiod-arm64/pigpiod -l
    else
        # 32-bit ARM architecture
        ./bin/pigpiod-arm/pigpiod -l
    fi

    sleep 1

    if pgrep -x "pigpiod" > /dev/null; then
        echo "pigpiod started successfully."
    else
        echo "pigpiod failed to start." >&2
        exit 1
    fi
fi

if [ "$ARCH" = "aarch64" ]; then
    # 64-bit ARM architecture
    exec ./bin/raspberry-pi-arm64 "$@"
else
    # 32-bit ARM architecture
    exec ./bin/raspberry-pi-arm "$@"
fi
