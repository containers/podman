#!/usr/bin/env bats

load helpers

function teardown() {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} rm -f -a"
    cleanup_test
}

function setup() {
    copy_images
}

@test "run pidns test" {

    ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${ALPINE}

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE}  sh -c 'echo \$\$'"
    echo $output
    [ "$status" -eq 0 ]
    pid=$(echo $output | tr -d '\r')
    [ $pid = "1" ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --pid=host ${ALPINE}  sh -c 'echo \$\$'"
    echo $output
    pid=$(echo $output | tr -d '\r')
    [ "$status" -eq 0 ]
    [ $pid !=  "1" ]

    run ${KPOD_BINARY} ${KPOD_OPTIONS} run --pid=badpid ${ALPINE} sh -c 'echo $$'
    echo $output
    [ "$status" -ne 0 ]
}
