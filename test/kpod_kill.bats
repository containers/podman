#!/usr/bin/env bats

load helpers

function teardown() {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm --force --all"
    cleanup_test
}

function setup() {
    copy_images
}

@test "kill a bogus container" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} kill foobar"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kill a running container by id" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} kill $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with TERM" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} kill -s TERM $ctr_id"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}

@test "kill a running container by name" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --name test1 -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} kill -s TERM test1"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}

@test "kill a running container by id with a bogus signal" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 9999"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} kill -s foobar $ctr_id"
    [ "$status" -eq 1 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --no-trunc"
    [ "$status" -eq 0 ]
}
