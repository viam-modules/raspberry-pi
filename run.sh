#!/bin/sh

#Install pigpio packets 
sudo apt-get install -qqy libpigpio-dev libpigpiod-if-dev pigpio

exec ./bin/raspberry-pi $1