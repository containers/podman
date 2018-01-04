#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "wait on a bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait 12343
    echo $output
    echo $status
    [ "$status" -eq 125 ]
}

@test "wait on a stopped container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} ls
    echo $output
    [ "$status" -eq 0 ]
    ctr_id=${output}
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait $ctr_id
    [ "$status" -eq 0 ]
}

@test "wait on a sleeping container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 10
    echo $output
    [ "$status" -eq 0 ]
    ctr_id=${output}
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait $ctr_id
    [ "$status" -eq 0 ]
}
