#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "run exit125 test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --foobar ${ALPINE} ls $tmp
    echo $output
    echo $status != 125
    [ $status -eq 125 ]
}

@test "run exit126 test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run ${ALPINE} foobar
    echo $output
    echo $status != 126
    [ "$status" -eq 126 ]
}

@test "run exit127 test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run ${ALPINE} /etc
    echo $output
    echo $status != 127
    [ "$status" -eq 127 ]
}

@test "run exit0 test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run ${ALPINE} ps
    echo $output
    echo $status != 0
    [ "$status" -eq 0 ]
}

@test "run exit50 test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run ${ALPINE} sh -c "exit 50"
    echo $output
    echo $status != 50
    [ "$status" -eq 50 ]
}
