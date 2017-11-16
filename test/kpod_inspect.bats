#!/usr/bin/env bats

load helpers

IMAGE="docker.io/library/busybox:latest"

function teardown() {
    cleanup_test
}

@test "kpod inspect image" {
    ${KPOD_BINARY} $KPOD_OPTIONS pull ${IMAGE}
    run bash -c "${KPOD_BINARY} $KPOD_OPTIONS inspect ${IMAGE} | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}


@test "kpod inspect non-existent container" {
    run ${KPOD_BINARY} $KPOD_OPTIONS inspect 14rcole/non-existent
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kpod inspect with format" {
    ${KPOD_BINARY} $KPOD_OPTIONS pull ${IMAGE}
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS inspect --format {{.ID}} ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    inspectOutput="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS images --no-trunc --quiet ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = "$inspectOutput" ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod inspect specified type" {
    ${KPOD_BINARY} $KPOD_OPTIONS pull ${IMAGE}
    run bash -c "${KPOD_BINARY} $KPOD_OPTIONS inspect --type image ${IMAGE} | python -m json.tool"
    echo "$output"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}
