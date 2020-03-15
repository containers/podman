#!/usr/bin/env bash
cc -E - > /dev/null 2> /dev/null << EOF
#include <systemd/sd-daemon.h>
EOF
if test $? -eq 0 ; then
	echo systemd
fi
