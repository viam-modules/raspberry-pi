#!/bin/sh

DEBIAN_VERSION=$(grep VERSION_CODENAME /etc/os-release | cut -d '=' -f 2)

echo "Detected Debian version: $DEBIAN_VERSION"

# If Debian Bullseye, update specific versions of pigpio, libpigpio1, and libpigpio-dev
if [ "$DEBIAN_VERSION" = "bullseye" ]; then
    echo "Updating pigpio packages for Bullseye..."
    apt-get update -qq
    apt-get install -qqy pigpio=1.79-1+rpt1 libpigpio1=1.79-1+rpt1 libpigpio-dev=1.79-1+rpt1
    if [ $? -ne 0 ]; then
        echo "Package installation failed due to dependency issues." >&2
        exit 1
    fi
else
    echo "Not Bullseye, skipping specific package updates."
    apt-get install -qqy pigpio
    if [ $? -ne 0 ]; then
        echo "Package installation failed." >&2
        exit 1
    fi
fi

# Enable pigpiod service
echo "Enabling and starting pigpiod service..."
systemctl enable pigpiod
systemctl start pigpiod

# Sleep for 1 second to allow the service to start
sleep 1

# Confirm pigpiod is running
if systemctl status pigpiod | grep -q "active (running)"; then
    echo "pigpiod is running successfully."
else
    echo "pigpiod failed to start." >&2
    exit 1
fi

echo "Installation and verification completed successfully!"

# Continue running the main program
exec ./bin/raspberry-pi "$@"