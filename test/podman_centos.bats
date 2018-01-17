#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "centos test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm  --privileged ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
    run echo "$output" | grep bin
    [ "$status" -eq 0 ]
}
