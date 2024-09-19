#!/bin/sh

# Function to check if a package is installed
is_installed() {
    dpkg -s "$1" > /dev/null 2>&1
}

# Install pigpio packages if not already installed
for pkg in libpigpiod-if2-1 pigpio; do
    if ! is_installed "$pkg"; then
        echo "Installing $pkg..."
        sudo apt-get install -qqy "$pkg"
    else
        echo "$pkg is already installed."
    fi
done

# start the pigpiod service
echo "Starting pigpiod service..."
sudo systemctl start pigpiod
sudo systemctl enable pigpiod

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
