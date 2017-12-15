#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "stop a bogus container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stop foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stop a running container by id" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stop $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
}

@test "stop a running container by name" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stop test1"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
}

@test "stop all containers" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test2 -d ${ALPINE} sleep 9999"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test3 -d ${ALPINE} sleep 9999"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stop -a -t 1"
    echo "$output"
    [ "$status" -eq 0 ]
}
