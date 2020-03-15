#!/usr/bin/bash

if test "$#" -ne 1; then
	echo "invalid arguments: usage: $0 path/to/binary"
	exit 1
fi

go tool nm -size "$1" \
	| awk 'NF==4 && $3=="t" {printf "%s\t\t%s\n", $2, $4}'
