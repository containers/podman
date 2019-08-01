#!/bin/bash

source $(dirname $0)/lib.sh

req_env_var TEST_REMOTE_CLIENT OS_RELEASE_ID GOSRC

cd $GOSRC

if [[ "$TEST_REMOTE_CLIENT" == "true" ]] && [[ -z "$CROSS_PLATFORM" ]]
then
    CROSS_PLATFORM=linux
fi

if [[ -n "$CROSS_PLATFORM" ]]
then
    echo "Compiling podman-remote release archive for ${CROSS_PLATFORM}"
    case "$CROSS_PLATFORM" in
        linux) ;&
        windows) ;&
        darwin)
            make podman-remote-${CROSS_PLATFORM}-release
            ;;
        *)
            die 1 "Unknown/unsupported cross-compile platform '$CROSS_PLATFORM'"
            ;;
    esac
else
    echo "Compiling release archive for $OS_RELEASE_ID"
    make podman-release
fi
