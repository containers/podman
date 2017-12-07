#!/usr/bin/env bats

load helpers

function setup() {
    prepare_network_conf
    copy_images
}

function teardown() {
    cleanup_test
}

@test "create a container based on local image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create $BB ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "create a container based on a remote image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create ${BB_GLIBC} ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "ensure short options" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create -dt ${BB_GLIBC} ls
    echo "$output"
    [ "$status" -eq 0 ]
}
