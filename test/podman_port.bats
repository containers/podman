#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "podman port all and latest" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -a -l
    echo "$output"
    echo "$status"
    [ "$status" -ne 0 ]
}

@test "podman port all and extra" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -a foobar
    echo "$output"
    echo "$status"
    [ "$status" -ne 0 ]
}

@test "podman port nginx" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt -P docker.io/library/nginx:latest
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -l
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -l 80
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -l 80/tcp
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} port -a
    echo "$output"
    [ "$status" -eq 0 ]
}
