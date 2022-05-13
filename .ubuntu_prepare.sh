#!/bin/bash
set -xeuo pipefail

export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH:$GOPATH/bin

runc=0
conmon=0
cni=0
podman_conf=0

conmon_source=/go/src/github.com/containers/conmon
cni_source=/go/src/github.com/containernetworking/plugins
runc_source=/go/src/github.com/opencontainers/runc
podman_source=/var/tmp/checkout

while getopts "cnrf" opt; do
    case "$opt" in
        c) conmon=1
            ;;
        f) podman_conf=1
            ;;
        n) cni=1
            ;;
        r) runc=1
            ;;
        *) echo "Nothing to do ... exiting."
            exit 0
            ;;
        esac
done

if [ $conmon -eq 1 ]; then
    # Build and install conmon from source
    echo "Building conmon ..."
    git clone https://github.com/containers/conmon $conmon_source
    cd $conmon_source && make install PREFIX=/usr
fi


if [ $cni -eq 1 ]; then
    # Build and install containernetworking plugins from source
    echo "Building containernetworking-plugins..."
    git clone https://github.com/containernetworking/plugins $cni_source
    cd $cni_source
    ./build.sh
    mkdir -p /usr/libexec/cni
    cp -v bin/* /usr/libexec/cni/
fi


if [ $runc -eq 1 ]; then
    # Build and install runc
    echo "Building runc..."
    git clone https://github.com/opencontainers/runc $runc_source
    cd $runc_source
    make install PREFIX=/usr
fi

if [ $podman_conf -eq 1 ]; then
    # Install various configuration files required by libpod

    # Install CNI conf file for podman
    mkdir -p /etc/cni/net.d
    cp -v $podman_source/cni/87-podman-bridge.conflist /etc/cni/net.d/

    # Install registries.conf
    mkdir -p /etc/containers
    cp -v $podman_source/test/registries.conf /etc/containers/
    cp -v $podman_source/test/policy.json /etc/containers/
fi
