#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

@test "wait on a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} wait 12343
    echo $output
    echo $status
    [ "$status" -eq 1 ]
}

@test "wait on a stopped container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    echo $output
    [ "$status" -eq 0 ]
    ctr_id=${output}
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} wait $ctr_id
    [ "$status" -eq 0 ]
}

@test "wait on a sleeping container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 10
    echo $output
    [ "$status" -eq 0 ]
    ctr_id=${output}
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} wait $ctr_id
    [ "$status" -eq 0 ]
}
