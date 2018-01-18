#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "test diff of image and parent" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS diff $BB
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test diff on non-existent layer" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS diff "abc123"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "test diff with json output" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS diff --format json $BB
    echo "$output"
    [ "$status" -eq 0 ]
}
