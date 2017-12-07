#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"
function setup() {
    prepare_network_conf
    copy_images
}

function teardown() {
    cleanup_test
}

@test "kpod ps with no containers" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps default" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps all flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps size flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --size"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps quiet flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    ctr_id="$output"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --quiet"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps latest flag" {
    run  bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --latest"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps last flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${BB} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls -s"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --last 2"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps no-trunc flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --no-trunc"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps namespace flag" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps --all --namespace"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps namespace flag and format flag = json" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps without namespace flag and format flag = json" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "kpod ps format flag = go template" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format 'table {{.ID}} {{.Image}} {{.Labels}}'"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - ancestor" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter ancestor=${ALPINE}"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - id" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} ls"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id=$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps filter flag - status" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 99"
    ctr_id="$output"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter status=running"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps short options" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 99"
    ctr_id="$output"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -aq"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps with mutually exclusive flags" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -d ${ALPINE} sleep 99"
    ctr_id="$output"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -aqs"
    echo "$output"
    [ "$status" -ne 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns -s"
    echo "$output"
    [ "$status" -ne 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns format {{.ID}}"
    echo "$output"
    [ "$status" -ne 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns --format json"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stop $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}
