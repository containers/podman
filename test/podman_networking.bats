#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "test network connection with default bridge" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait --latest
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test network connection with host" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt --network host ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait --latest
    echo "$output"
    [ "$status" -eq 0 ]
}
