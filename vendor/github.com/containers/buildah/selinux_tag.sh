#!/bin/bash
if pkg-config libselinux 2> /dev/null ; then
	echo selinux
fi
