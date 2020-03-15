#!/usr/bin/bash

if [ -z "$WORK" ]
then
	echo "WORK environment variable must be set"
	exit 1
fi

grep --no-filename packagefile $WORK/**/importcfg \
	| awk '{ split($2, data, "="); printf "%s ", data[1]; system("du -sh " data[2]) }' \
	| awk '{ printf "%s %s\n", $2, $1 }' \
	| sort -u | sort -rh
