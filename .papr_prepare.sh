#!/bin/bash
set -xeuo pipefail

# Provision the previous version of podman to use for our tests by default
cat >/etc/yum.repos.d/crio-copr.repo <<EOF
[crio-copr]
baseurl=https://copr-be.cloud.fedoraproject.org/results/baude/Upstream_CRIO_Family/fedora-27-x86_64/
gpgcheck=0
EOF

pkg_install() {
    if test -x /usr/bin/yum; then
        yum -y install "$@"
    else
        rpm-ostree install "$@" && rpm-ostree ex livefs
    fi
}
pkg_install podman buildah

DIST=${DIST:=Fedora}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:=podman}
IMAGE=fedorapodmanbuild
PYTHON=python3
if [[ ${DIST} != "Fedora" ]]; then
    IMAGE=centospodmanbuild
    PYTHON=python
fi

# Build the test image
${CONTAINER_RUNTIME} build -t ${IMAGE} -f Dockerfile.${DIST} .

# Run the tests
${CONTAINER_RUNTIME} run --rm --privileged --net=host -v $PWD:/go/src/github.com/projectatomic/libpod --workdir /go/src/github.com/projectatomic/libpod -e PYTHON=$PYTHON -e STORAGE_OPTIONS="--storage-driver=vfs" -e CRIO_ROOT="/go/src/github.com/projectatomic/libpod" -e PODMAN_BINARY="/usr/bin/podman" -e CONMON_BINARY="/usr/libexec/crio/conmon" -e DIST=$DIST $IMAGE sh .papr.sh
