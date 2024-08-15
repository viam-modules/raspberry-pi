/*
    pi.h: Header file for pi.c
*/
#pragma once

// interruptCallback calls through to the go linked interrupt callback.
int setupInterrupt(int pi, int gpio);
int teardownInterrupt(unsigned int callback_id);