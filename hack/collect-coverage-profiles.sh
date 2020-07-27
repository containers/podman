#!/bin/bash

# Disable modules to prevent `go cover` from going south.
# It's
export GO111MODULE=off

PROFILE="coverprofile.final"
profiles=`find ${COVERAGE_PATH} -type f`

echo "mode: count" > $PROFILE

echo "* Assembling coverage profiles into $PROFILE"
for profile in $profiles
do
	tail -n +2 $profile >> $PROFILE
done

echo "* Generating html coverage file ${PROFILE}.html"
go tool cover -html=${PROFILE} -o ${PROFILE}.html

echo "* Analyzing coverage profile into ${PROFILE}.functions"
go tool cover -func=${PROFILE} > ${PROFILE}.functions

cat ${PROFILE}.functions | sed -n 's/\(total:\).*\([0-9][0-9].[0-9]\)/\1 \2/p'
