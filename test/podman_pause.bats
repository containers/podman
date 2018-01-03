#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "pause a bogus container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} pause foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "unpause a bogus container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} unpause foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "pause a created container by id" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id=`echo "$output" | tail -n 1`
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} pause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} unpause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm -f $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "pause a running container by id" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id=`echo "$output" | tail -n 1`
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} pause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} unpause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm -f $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "unpause a running container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id=`echo "$output" | tail -n 1`
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} unpause $ctr_id"
    echo "$output"
    [ "$status" -eq 1 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm -f $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove a paused container by id" {
    skip "Test needs to wait for --force to work for podman rm"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id=`echo "$output" | tail -n 1`
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} pause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 1 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm --force $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stop a paused container created by id" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id=`echo "$output" | tail -n 1`
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} pause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stop $ctr_id"
    echo "$output"
    [ "$status" -eq 1 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} unpause $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} ps -a --filter id=$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm -f $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}
