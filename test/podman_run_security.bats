#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "run privileged test" {
    cap=$(grep CapEff /proc/self/status | cut -f2 -d":")

    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --privileged ${ALPINE}  grep CapEff /proc/self/status
    echo $output
    [ "$status" -eq 0 ]
    containercap=$(echo $output | tr -d '\r'| cut -f2 -d":")
    [ $containercap = $cap ]

    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --cap-add all ${ALPINE}  grep CapEff /proc/self/status
    echo $output
    [ "$status" -eq 0 ]
    containercap=$(echo $output | tr -d '\r'| cut -f2 -d":")
    [ $containercap = $cap ]

    cap=$(grep CapAmb /proc/self/status | cut -f2 -d":")
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --cap-drop all ${ALPINE}  grep CapEff /proc/self/status
    echo $output
    [ "$status" -eq 0 ]
    containercap=$(echo $output | tr -d '\r'| cut -f2 -d":")
    [ $containercap = $cap ]
}
