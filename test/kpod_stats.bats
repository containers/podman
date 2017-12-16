#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "stats should run with no containers" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats with bogus container id" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream  123
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stats on a running container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t ${ALPINE} sleep 99
    ctr_id="$output"
    echo "$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream  $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats on a running container no id" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t ${ALPINE} sleep 99
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats on all containers" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t ${ALPINE} ls
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream -a
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats only output IDs" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t ${ALPINE} sleep 99
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream --format "{{.Container}}"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "stats json output" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d -t ${ALPINE} ls
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream -a --format json | python -m json.tool"
    echo "$output"
    [ "$status" -eq 0 ]
}
