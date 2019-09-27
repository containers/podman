#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var TEST_REMOTE_CLIENT OS_RELEASE_ID GOSRC

cd $GOSRC

if [[ "$TEST_REMOTE_CLIENT" == "true" ]] && [[ -z "$CROSS_PLATFORM" ]]
then
    CROSS_PLATFORM=linux
fi

if [[ -n "$CROSS_PLATFORM" ]]
then
    # Will fail if $CROSS_PLATFORM is unsupported cross-compile $GOOS value
    make podman-remote-${CROSS_PLATFORM}-release

    echo "Compiling podman-remote release archive for ${CROSS_PLATFORM}"
    if [[ "$CROSS_PLATFORM" == "windows" ]]
    then
        # TODO: Remove next line, part of VM images next time they're built.
        dnf install -y libmsi1 msitools pandoc
        make podman.msi
    fi
else
    echo "Compiling release archive for $OS_RELEASE_ID"
    make podman-release
fi

echo "Preserving build details for later use."
mv -v release.txt actual_release.txt  # Another 'make' during testing could overwrite it
