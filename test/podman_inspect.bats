#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "podman inspect image" {
    run bash -c "${PODMAN_BINARY} $PODMAN_OPTIONS inspect ${ALPINE} | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman inspect non-existent container" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS inspect 14rcole/non-existent
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "podman inspect with format" {
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS inspect --format {{.ID}} ${ALPINE}
    echo "$output"
    [ "$status" -eq 0 ]
    inspectOutput="$output"
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS images --no-trunc --quiet ${ALPINE}
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = "$inspectOutput" ]
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman inspect specified type" {
    run bash -c "${PODMAN_BINARY} $PODMAN_OPTIONS inspect --type image ${ALPINE} | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman inspect container with size" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} create ${BB} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} $PODMAN_OPTIONS inspect --size $ctr_id | python -m json.tool | grep SizeRootFs"
    echo "$output"
    [ "$status" -eq 0 ]
}
