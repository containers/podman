#!/usr/bin/bash

if test "$#" -ne 1; then
	echo "invalid arguments: usage: $0 path/to/binary"
	exit 1
fi

DATA=$(go tool nm -size "$1" \
	| awk 'NF==4 {printf "%s\t%s\t%s\n", $2, $3, $4}' \
	| grep -v -P "\t_\t" \
	| grep -P "\tt\t" \
	| awk ' {printf "%s\t\t%s\n", $1, $3} ' \
	)

echo "$DATA"
