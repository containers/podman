#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "exec into a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} exec foobar ls
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "exec without command should fail" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} exec foobar
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "exec simple command" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t --name foobar1 ${ALPINE} sleep 60
    run ${KPOD_BINARY} ${KPOD_OPTIONS} exec foobar1 ls
    echo "$output"
    [ "$status" -eq 0 ]
}
