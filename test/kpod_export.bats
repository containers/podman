#!/usr/bin/env bats

load helpers

function teardown() {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm -f -a"
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod export output flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    rm -f container.tar
}
