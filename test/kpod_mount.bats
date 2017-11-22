#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "mount" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount --notruncate | grep $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} unmount $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount --format=json | python -m json.tool | grep $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} unmount $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}
