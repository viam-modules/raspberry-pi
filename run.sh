#!/bin/sh


WHATPI=$(awk '{print $3}' /proc/device-tree/model)
if [ "$WHATPI" = "5" ]; then
    exec ./bin/raspberry-pi-arm64 "$@" "pi5-detected"
fi
# install packages 
apt-get install -qqy pigpio

# enable pigpiod
systemctl enable pigpiod

# start the pigpiod service
echo "Starting pigpiod service..."
systemctl start pigpiod

sleep 1 

# Confirm pigpiod is running
if systemctl status pigpiod | grep -q "active (running)"; then
    echo "pigpiod is running successfully."
else
    echo "pigpiod failed to start." >&2
    exit 1
fi

echo "Installation and verification completed successfully!"

# Determine architecture and execute the correct binary
ARCH=$(uname -m)

if [ "$ARCH" = "aarch64" ]; then
    # 64-bit ARM architecture
    exec ./bin/raspberry-pi-arm64 "$@"
else
    # 32-bit ARM architecture
    exec ./bin/raspberry-pi-arm "$@"
fi
