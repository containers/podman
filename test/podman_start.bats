#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "start bogus container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} start 1234
    echo "$output"
    [ "$status" -eq 125 ]
}

@test "start single container by id" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create -d ${ALPINE} ls
    ctr_id=${output}
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} start $ctr_id
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start single container by name" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create -d --name foobar99 ${ALPINE} ls
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} start foobar
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start multiple containers" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create -d ${ALPINE} ls
    ctr1_id=${output}
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create -d ${ALPINE} ls
    ctr1_id2=${output}
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} start $ctr1_id $ctr2_id
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start multiple containers -- attach should fail" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --name foobar1 -d ${ALPINE} ls
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --name foobar2 -d ${ALPINE} ls
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} start -a foobar1 foobar2
    echo "$output"
    [ "$status" -eq 125 ]
}
