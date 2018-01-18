#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "attach to a bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} attach foobar
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "attach to non-running container" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --name foobar -d -i ${ALPINE} ls
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} attach foobar
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "attach to multiple containers" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name foobar1 -d -i ${ALPINE} /bin/sh
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name foobar2 -d -i ${ALPINE} /bin/sh
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} attach foobar1 foobar2
    echo "$output"
    [ "$status" -eq 125 ]
}
