#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "stop a bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop foobar
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "stop a running container by id" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop $ctr_id
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
}

@test "stop a running container by name" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop test1
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
}

@test "stop all containers" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test2 -d ${ALPINE} sleep 9999
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test3 -d ${ALPINE} sleep 9999
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop -a -t 1
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stop a container with latest" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop -t 1 -l
    echo "$output"
    [ "$status" -eq 0 ]
}
