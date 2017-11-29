#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "remove a stopped container" {
    run ${KPOD_BINARY} $KPOD_OPTIONS run -d ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "refuse to remove a running container" {
    run ${KPOD_BINARY} $KPOD_OPTIONS run -d ${ALPINE} sleep 15
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash ${KPOD_BINARY} $KPOD_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "remove a created container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove a running container" {
    run ${KPOD_BINARY} $KPOD_OPTIONS run -d ${ALPINE} sleep 15
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove all containers" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls -l
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB true
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB whoami
    run ${KPOD_BINARY} $KPOD_OPTIONS rm -a
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove all containers with one running" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls -l
    ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB whoami
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 30
    run ${KPOD_BINARY} $KPOD_OPTIONS rm -a -f
    echo "$output"
    [ "$status" -eq 0 ]
}
