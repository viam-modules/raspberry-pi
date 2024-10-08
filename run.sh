#!/bin/sh

# if libpigpio is installed then we install pigpio 
# with the same version of libpigpio
if ! dpkg -s pigpio >/dev/null 2>&1; then 

    # check if libpigpio is installed
    if dpkg -s libpigpio-dev >/dev/nul 2>&1; then 
        echo "libpigpio-dev is installed. Checking version"
        
        PIGPIO_VERSION=$(dpkg -s libpigpio-dev | grep '^Version:' | awk '{print $2}')
        echo "found libpigpio version: $PIGPIO_VERSION"
        apt-get instal pigpio="$PIGPIO_VERSION"
    else
        apt-get install pigpio
    fi
else 
    echo "pigpio is already installed"
fi

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

exec ./bin/raspberry-pi "$@"
