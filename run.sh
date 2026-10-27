#!/bin/sh

# Pigpio client libraries
DEBIAN_FRONTEND=noninteractive apt install -qqy libpigpiod-if2-1 2>&1

# RPI5 doesn't need pigpiod
WHATPI=$(awk '{print $3}' /proc/device-tree/model)
if [ "$WHATPI" = "5" ]; then
    exec ./bin/raspberry-pi-arm64 "$@" "pi5-detected"
fi

ARCH=$(uname -m)

if [ "$ARCH" = "aarch64" ]; then
    PIGPIOD=./bin/pigpiod-arm64/pigpiod
else
    PIGPIOD=./bin/pigpiod-arm/pigpiod
fi

# pigpiod listens on PIGPIO_PORT (default 8888). The module binary reads the same
# env var when connecting, so a user can override it in the module's env config.
PIGPIO_PORT="${PIGPIO_PORT:-8888}"

# Prints a description of whatever is listening on the given TCP port, or nothing if it is free.
port_listener() {
    if command -v ss > /dev/null 2>&1; then
        ss -Hltnp "sport = :$1" 2>/dev/null
    fi
}

# Prints the TCP port a running pigpiod is listening on, or nothing if it cannot be determined.
pigpiod_port() {
    if command -v ss > /dev/null 2>&1; then
        ss -Hltnp 2>/dev/null | grep '"pigpiod"' | awk '{n = split($4, a, ":"); print a[n]; exit}'
    fi
}

# Check if pigpiod is already running.
# NOTE: it may be running as a service or locally started process. No attempt is made to control it.
if pgrep -x "pigpiod" > /dev/null; then
    # It may have been started on a fallback port by a previous run of this script.
    RUNNING_PORT=$(pigpiod_port)
    if [ -n "$RUNNING_PORT" ]; then
        PIGPIO_PORT=$RUNNING_PORT
    fi
    echo "pigpiod is already running, not explicitly starting it (using port $PIGPIO_PORT)."
else
    echo "pigpiod is not running, starting..."

    # If another process already holds the port, pigpiod cannot bind it. Rather than
    # failing, log who owns it and fall back to the next free port.
    LISTENER=$(port_listener "$PIGPIO_PORT")
    if [ -n "$LISTENER" ]; then
        echo "port $PIGPIO_PORT is already in use by another process: $LISTENER"
        REQUESTED_PORT=$PIGPIO_PORT
        PORT=$((PIGPIO_PORT + 1))
        while [ "$PORT" -le $((REQUESTED_PORT + 20)) ]; do
            if [ -z "$(port_listener "$PORT")" ]; then
                PIGPIO_PORT=$PORT
                break
            fi
            PORT=$((PORT + 1))
        done
        if [ "$PIGPIO_PORT" = "$REQUESTED_PORT" ]; then
            echo "no free port found in range $((REQUESTED_PORT + 1))-$((REQUESTED_PORT + 20)) for pigpiod." >&2
            exit 1
        fi
        echo "starting pigpiod on fallback port $PIGPIO_PORT instead."
    fi

    "$PIGPIOD" -l -p "$PIGPIO_PORT"

    sleep 1

    if pgrep -x "pigpiod" > /dev/null; then
        echo "pigpiod started successfully on port $PIGPIO_PORT."
    else
        echo "pigpiod failed to start on port $PIGPIO_PORT." >&2
        exit 1
    fi
fi

export PIGPIO_PORT

if [ "$ARCH" = "aarch64" ]; then
    # 64-bit ARM architecture
    exec ./bin/raspberry-pi-arm64 "$@"
else
    # 32-bit ARM architecture
    exec ./bin/raspberry-pi-arm "$@"
fi
