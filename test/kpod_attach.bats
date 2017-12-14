#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "attach to a bogus container" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} attach foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "attach to non-running container" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} create --name foobar -d -i ${ALPINE} ls
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} attach foobar"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "attach to multiple containers" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run --name foobar1 -d -i ${ALPINE} /bin/sh
    ${KPOD_BINARY} ${KPOD_OPTIONS} run --name foobar2 -d -i ${ALPINE} /bin/sh
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} attach foobar1 foobar2"
    echo "$output"
    [ "$status" -eq 1 ]
}
