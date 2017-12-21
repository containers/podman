#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "test network connection with default bridge" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -dt ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test network connection with host" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -dt --network host ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
}
