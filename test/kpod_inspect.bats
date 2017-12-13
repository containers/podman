#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod inspect image" {
    run bash -c "${KPOD_BINARY} $KPOD_OPTIONS inspect ${ALPINE} | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod inspect non-existent container" {
    run ${KPOD_BINARY} $KPOD_OPTIONS inspect 14rcole/non-existent
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kpod inspect with format" {
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS inspect --format {{.ID}} ${ALPINE}
    echo "$output"
    [ "$status" -eq 0 ]
    inspectOutput="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS images --no-trunc --quiet ${ALPINE}
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = "$inspectOutput" ]
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod inspect specified type" {
    run bash -c "${KPOD_BINARY} $KPOD_OPTIONS inspect --type image ${ALPINE} | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod inspect container with size" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} create ${BB} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${KPOD_BINARY} $KPOD_OPTIONS inspect --size $ctr_id | python -m json.tool | grep SizeRootFs"
    echo "$output"
    [ "$status" -eq 0 ]
}
