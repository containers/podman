#!/usr/bin/env bash

set -exo pipefail

whoami

ls -al

uname -r

rpm -q aardvark-dns buildah conmon container-selinux containers-common crun netavark passt podman skopeo slirp4netns systemd

make -C $PODMAN_SOURCE_DIR $1
