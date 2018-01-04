#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "top without container name or id" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} top
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "top a bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} top foobar
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "top non-running container by id with defaults" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create -d ${ALPINE} sleep 60
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} top $ctr_id"
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "top running container by id with defaults" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt ${ALPINE} /bin/sh
    [ "$status" -eq 0 ]
    ctr_id="$output"
    echo $ctr_id
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} top $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "top running container by id with ps opts" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 60
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} top $ctr_id -o fuser,f,comm,label"
    echo "$output"
    [ "$status" -eq 0 ]
}
