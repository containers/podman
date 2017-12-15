#!/usr/bin/env bats

load helpers

function setup() {
    prepare_network_conf
    copy_images
}

function teardown() {
    cleanup_test
}

@test "remove a stopped container" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS run -d ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "refuse to remove a running container" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS run -d ${ALPINE} sleep 15
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash ${PODMAN_BINARY} $PODMAN_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "remove a created container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove a running container" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS run -d ${ALPINE} sleep 15
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove all containers" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls -l
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB true
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB whoami
    run ${PODMAN_BINARY} $PODMAN_OPTIONS rm -a
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "remove all containers with one running with short options" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB ls -l
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create $BB whoami
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d ${ALPINE} sleep 30
    run ${PODMAN_BINARY} $PODMAN_OPTIONS rm -af
    echo "$output"
    [ "$status" -eq 0 ]
}
