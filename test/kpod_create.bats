#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm -f `${KPOD_BINARY} ${KPOD_OPTIONS} ps -a -q`"
    cleanup_test
}

@test "create a container based on local image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "create a container based on a remote image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create ${BB_GLIBC} ls
    echo "$output"
    [ "$status" -eq 0 ]
}
