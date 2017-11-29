#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"
function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}

@test "kpod ps with no containers" {
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps default" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps all flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps size flag" {
    skip "Need size flag implemented in container backend"
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --size
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps quiet flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --quiet
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps latest flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --latest
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps last flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${BB} ls
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls -s
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --last 2
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps no-trunc flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --no-trunc
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps namespace flag" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --all --namespace
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps namespace flag and format flag = json" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps without namespace flag and format flag = json" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "kpod ps format flag = go template" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format "table {{.ID}} {{.Image}} {{.Labels}}"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - ancestor" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter ancestor=${ALPINE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - id" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - status" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 99
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter status=running
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop $ctr_id
}
