#!/bin/bash
set -xeuo pipefail

DIST=$(cat /etc/redhat-release  | awk '{print $1}')
IMAGE=fedorapodmanbuild
PYTHON=python3
if [[ ${DIST} != "Fedora" ]]; then
    IMAGE=centospodmanbuild
    PYTHON=python
fi

# Build the test image
docker build -t ${IMAGE} -f Dockerfile.${DIST} .

# Run the tests
docker run --rm --privileged -v $PWD:/go/src/github.com/projectatomic/libpod --workdir /go/src/github.com/projectatomic/libpod -e PYTHON=$PYTHON -e STORAGE_OPTIONS="--storage-driver=vfs" -e CRIO_ROOT="/go/src/github.com/projectatomic/libpod" -e PODMAN_BINARY="/usr/bin/podman" -e CONMON_BINARY="/usr/libexec/crio/conmon" -e DIST=$DIST $IMAGE sh .papr.sh
