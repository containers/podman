#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "podman export output flag" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    rm -f container.tar
}
