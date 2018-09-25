#!/bin/bash
set -xeuo pipefail

export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH
export GOSRC=/$GOPATH/src/github.com/containers/libpod

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

while getopts "bituv" opt; do
    case "$opt" in
    b) build=1
       ;;
    i) install=1
       ;;
    t) integrationtest=1
       ;;
    u) unittest=1
       ;;
    v) validate=1
       ;;
    esac
done

# If no options are passed, do everything
if [ $OPTIND -eq 1 ]; then 
    build=1
    install=1
    integrationtest=1
    unittest=1
    validate=1
fi

# Validate
if [ $validate -eq 1 ]; then
    # PAPR adds a merge commit, for testing, which fails the
    # short-commit-subject validation test, so tell git-validate.sh to only check
    # up to, but not including, the merge commit.
    export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
    export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) $($GOSRC/hack/selinux_tag.sh)"

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
    make TAGS="${TAGS}"
fi

# Install Podman
if [ $install -eq 1 ]; then
    make install.tools TAGS="${TAGS}"
    make TAGS="${TAGS}" install PREFIX=/usr ETCDIR=/etc
fi

# Run integration tests
if [ $integrationtest -eq 1 ]; then 
    make TAGS="${TAGS}" test-binaries
    SKIP_USERNS=1 GOPATH=/go make localintegration
fi


exit 0

