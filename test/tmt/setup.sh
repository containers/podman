#!/usr/bin/env bash

set -exo pipefail

uname -r

loginctl enable-linger "$ROOTLESS_USER"

rm -rf /home/$ROOTLESS_USER/.local/share/containers

rpm -q \
    aardvark-dns \
    buildah \
    conmon \
    container-selinux \
    containers-common \
    crun \
    netavark \
    passt \
    podman \
    podman-tests \
    skopeo \
    slirp4netns \
    systemd
