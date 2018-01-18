#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kill a bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill foobar
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kill a running container by id" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill $ctr_id
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with TERM" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s TERM $ctr_id
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc
    [ "$status" -eq 0 ]
}

@test "kill a running container by name" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s TERM test1
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with a bogus signal" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -s foobar $ctr_id
    [ "$status" -eq 125 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} ps --no-trunc
    [ "$status" -eq 0 ]
}

@test "kill the latest container run" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 9999
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} kill -l
    echo "$output"
    [ "$status" -eq 0 ]
}
