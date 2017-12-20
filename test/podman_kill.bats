#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kill a bogus container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} kill foobar"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kill a running container by id" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} kill $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with TERM" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s TERM $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}

@test "kill a running container by name" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s TERM test1"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with a bogus signal" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s foobar $ctr_id"
    [ "$status" -eq 1 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}
