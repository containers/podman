#!/usr/bin/env bats

load helpers

function setup() {
    prepare_network_conf
    copy_images
}

function teardown() {
    cleanup_test
}

@test "test diff of image and parent" {
    run ${KPOD_BINARY} $KPOD_OPTIONS diff $BB
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test diff on non-existent layer" {
    run ${KPOD_BINARY} $KPOD_OPTIONS diff "abc123"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "test diff with json output" {
    # run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} diff --format json $IMAGE | python -m json.tool"
    run ${KPOD_BINARY} $KPOD_OPTIONS diff --format json $BB
    echo "$output"
    [ "$status" -eq 0 ]
}
