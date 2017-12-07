#!/usr/bin/env bats

load helpers

function setup() {
    prepare_network_conf
    copy_images
}

function teardown() {
    cleanup_test
}

@test "start bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} start 1234
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "start single container by id" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create -d ${ALPINE} ls
    ctr_id=${output}
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} start $ctr_id
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start single container by name" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} create -d --name foobar99 ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} start foobar
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start multiple containers" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create -d ${ALPINE} ls
    ctr1_id=${output}
    run ${KPOD_BINARY} ${KPOD_OPTIONS} create -d ${ALPINE} ls
    ctr1_id2=${output}
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} start $ctr1_id $ctr2_id
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "start multiple containers -- attach should fail" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} create --name foobar1 -d ${ALPINE} ls
    ${KPOD_BINARY} ${KPOD_OPTIONS} create --name foobar2 -d ${ALPINE} ls
    run ${KPOD_BINARY} ${KPOD_OPTIONS} start -a foobar1 foobar2
    echo "$output"
    [ "$status" -eq 1 ]
}
