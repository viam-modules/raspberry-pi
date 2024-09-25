#!/bin/sh

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

exec ./bin/raspberry-pi "$@"
