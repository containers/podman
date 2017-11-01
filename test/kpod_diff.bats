#!/usr/bin/env bats

load helpers

IMAGE="alpine:latest"

function teardown() {
    cleanup_test
}

@test "test diff of image and parent" {
    run ${KPOD_BINARY} $KPOD_OPTIONS pull $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} $KPOD_OPTIONS diff $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test diff on non-existent layer" {
    run ${KPOD_BINARY} $KPOD_OPTIONS diff "abc123"
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "test diff with json output" {
    run ${KPOD_BINARY} $KPOD_OPTIONS pull $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
    # run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} diff --format json $IMAGE | python -m json.tool"
    run ${KPOD_BINARY} $KPOD_OPTIONS diff --format json $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
}
