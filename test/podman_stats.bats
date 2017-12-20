#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "stats should run with no containers" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats with bogus container id" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream  123
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stats on a running container" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d -t ${ALPINE} sleep 99
    ctr_id="$output"
    echo "$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream  $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats on a running container no id" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d -t ${ALPINE} sleep 99
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats on all containers" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d -t ${ALPINE} ls
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream -a
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats only output IDs" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d -t ${ALPINE} sleep 99
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream --format "{{.Container}}"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats json output" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d -t ${ALPINE} ls
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} stats --no-stream -a --format json | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}
