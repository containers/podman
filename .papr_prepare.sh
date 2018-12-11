#!/bin/bash
set -xeuo pipefail

DIST=${DIST:=Fedora}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:=docker}
IMAGE=fedorapodmanbuild
PYTHON=python3
if [[ ${DIST} != "Fedora" ]]; then
    IMAGE=centospodmanbuild
    PYTHON=python
fi

# Since CRIU 3.11 has been pushed to Fedora 28 the checkpoint/restore
# test cases are actually run. As CRIU uses iptables to lock and unlock
# the network during checkpoint and restore it needs the following two
# modules loaded.
modprobe ip6table_nat || :
modprobe iptable_nat || :

# Build the test image
${CONTAINER_RUNTIME} build -t ${IMAGE} -f Dockerfile.${DIST} . 2>build.log

# Run the tests
${CONTAINER_RUNTIME} run --rm --privileged --net=host -v $PWD:/go/src/github.com/containers/libpod:Z --workdir /go/src/github.com/containers/libpod -e CGROUP_MANAGER=cgroupfs -e PYTHON=$PYTHON -e STORAGE_OPTIONS="--storage-driver=fuse-overlayfs" -e CRIO_ROOT="/go/src/github.com/containers/libpod" -e PODMAN_BINARY="/usr/bin/podman" -e CONMON_BINARY="/usr/libexec/podman/conmon" -e DIST=$DIST -e CONTAINER_RUNTIME=$CONTAINER_RUNTIME $IMAGE sh ./.papr.sh -b -i -t
