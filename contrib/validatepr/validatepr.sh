#!/bin/bash

set -x

#
# This script is intended to help developers contribute to the podman project. It
# checks various pre-CI checks like building, linting, man-pages, etc.  It is meant
# to be run in a specific container environment.
#

# build all require incantations of podman
echo "Building windows ..."
GOOS=windows CGO_ENABLED=0 go build -tags "$REMOTETAGS" -o bin/test.windows ./cmd/podman
echo "Building darwin..."
GOOS=darwin CGO_ENABLED=0 go build -tags "$REMOTETAGS" -o bin/test.darwin ./cmd/podman

# build podman
echo "Building podman binaries ..."
make binaries


echo "Running validation tooling ..."
make validate
