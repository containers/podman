#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "run pidns test" {

    ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${ALPINE}

    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run ${ALPINE}  sh -c 'echo \$\$'"
    echo $output
    [ "$status" -eq 0 ]
    pid=$(echo $output | tr -d '\r')
    [ $pid = "1" ]

    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --pid=host ${ALPINE}  sh -c 'echo \$\$'"
    echo $output
    pid=$(echo $output | tr -d '\r')
    [ "$status" -eq 0 ]
    [ $pid !=  "1" ]

    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --pid=badpid ${ALPINE} sh -c 'echo $$'
    echo $output
    [ "$status" -ne 0 ]
}

@test "run ipcns test" {

    ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${ALPINE}

    tmp=$(mktemp /dev/shm/foo.XXXXX)
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --ipc=host ${ALPINE} ls $tmp
    echo $output
    out=$(echo $output | tr -d '\r')
    [ "$status" -eq 0 ]
    [ $out !=  $tmp ]

    rm -f $tmp

    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --ipc=badpid ${ALPINE} sh -c 'echo $$'
    echo $output
    [ "$status" -ne 0 ]
}
