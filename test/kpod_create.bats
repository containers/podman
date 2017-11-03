#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

ALPINE="docker.io/library/alpine:latest"

@test "create a container based on local image" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create docker.io/library/busybox:latest ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "create a container based on a remote image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
}
