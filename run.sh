#!/bin/sh

# Function to check if a package is installed
is_installed() {
    dpkg -s "$1" > /dev/null 2>&1
}

# Install pigpio packages if not already installed
for pkg in libpigpio-dev libpigpiod-if-dev pigpio; do
    if ! is_installed "$pkg"; then
        echo "Installing $pkg..."
        sudo apt-get install -qqy "$pkg"
    else
        echo "$pkg is already installed."
    fi
done


exec ./bin/raspberry-pi "$@"