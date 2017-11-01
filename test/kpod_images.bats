#!/usr/bin/env bats

load helpers

IMAGE="debian:6.0.10"

function teardown() {
    cleanup_test
}

@test "kpod images" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod images test valid json" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} images --format json
    echo "$output" | python -m json.tool
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod images check name json output" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} images --format json
    echo "$output"
    [ "$status" -eq 0 ]
    name=$(echo $output | python -c 'import sys; import json; print(json.loads(sys.stdin.read())[0])["names"][0]')
    [ "$name" = "docker.io/library/${IMAGE}" ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}
