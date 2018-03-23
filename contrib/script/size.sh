#!/bin/sh

: "${GOPATH?Need to set GOPATH}"

cd cmd/podman/ && eval `go build -work -a 2>&1` && find $WORK -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g
