#!/bin/bash
set -xeo pipefail

export GOPATH=/var/tmp/go
export PATH=$HOME/gopath/bin:$PATH:$GOPATH/bin
export GOSRC=$GOPATH/src/github.com/containers/libpod

DIST=${DIST:=""}
CONTAINER_RUNTIME=${DIST:=""}

source /etc/os-release

INTEGRATION_TEST_ENVS=""

# For all distributions not Fedora, we need to skip USERNS tests
# for now.
if [ "${ID}" != "fedora" ] || [ "${CONTAINER_RUNTIME}" != "" ]; then
    INTEGRATION_TEST_ENVS="SKIP_USERNS=1"
fi

pwd

# -i install
# -b build
# -t integration test
# -u unit test
# -v validate

build=0
install=0
integrationtest=0
unittest=0
validate=0
options=0
remote=0
install_tools_made=0

while getopts "bituv" opt; do
    case "$opt" in
    b) build=1
       options=1
       ;;
    i) install=1
       options=1
       ;;
    t) integrationtest=1
       options=1
       ;;
    u) unittest=1
       options=1
       ;;
    v) validate=1
       options=1
       ;;
    esac
done

# The TEST_REMOTE_CLIENT environment variable decides whether
# to test varlinke
if [[ "$TEST_REMOTE_CLIENT" == "true" ]]; then
    remote=1
fi

# If no options are passed, do everything
if [ $options -eq 0 ]; then
    build=1
    install=1
    integrationtest=1
    unittest=1
    validate=1
fi

# Make Install tools function used by multiple sections below
make_install_tools () {
    # Only make the install tools once
    if [ $install_tools_made -eq 0 ]; then
        make install.tools TAGS="${TAGS}"
    fi
    install_tools_made=1
}

CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-none}

if [ "${CONTAINER_RUNTIME}" == "none" ]; then
    mkdir -p /$GOPATH/src/github.com/containers/
    mv /var/tmp/checkout $GOSRC
    cd $GOSRC
    pwd
fi


export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/selinux_tag.sh)"

# Validate
if [ $validate -eq 1 ]; then
    make_install_tools
    # PAPR adds a merge commit, for testing, which fails the
    # short-commit-subject validation test, so tell git-validate.sh to only check
    # up to, but not including, the merge commit.
    export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
    make gofmt TAGS="${TAGS}"

    # Only check lint and gitvalidation on more recent
    # distros with updated git and tooling
    if [[ ${DIST} == "Fedora" ]]; then
        HEAD=$GITVALIDATE_TIP make -C $GOSRC .gitvalidation TAGS="${TAGS}"
        make lint
    fi
fi

# Unit tests
if [ $unittest -eq 1 ]; then
    make localunit TAGS="${TAGS}"
fi

# Make Podman
if [ $build -eq 1 ]; then
    make_install_tools
    make TAGS="${TAGS}" GOPATH=$GOPATH
    make podman-remote TAGS="${TAGS}" GOPATH=$GOPATH
fi

# Install Podman
if [ $install -eq 1 ]; then
    make_install_tools
    make TAGS="${TAGS}" install.bin PREFIX=/usr ETCDIR=/etc
    make TAGS="${TAGS}" install.man PREFIX=/usr ETCDIR=/etc
    make TAGS="${TAGS}" install.cni PREFIX=/usr ETCDIR=/etc
    make TAGS="${TAGS}" install.systemd PREFIX=/usr ETCDIR=/etc
fi

# Run integration tests
if [ $integrationtest -eq 1 ]; then
    make TAGS="${TAGS}" test-binaries
    make varlink_generate
    make localintegration $INTEGRATION_TEST_ENVS
    if [ $remote -eq 1 ]; then
        make remoteintegration $INTEGRATION_TEST_ENVS
    fi
fi
