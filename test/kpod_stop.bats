#!/usr/bin/env bats

load helpers

function teardown() {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm --force --all"
    cleanup_test
}

function setup() {
    copy_images
}

@test "stop a bogus container" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stop a running container by id" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
}

@test "stop a running container by name" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop test1"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
}
