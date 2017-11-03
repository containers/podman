#!/usr/bin/env bats

load helpers

IMAGE="busybox"

function teardown() {
    cleanup_test
}

@test "kpod images" {
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod images test valid json" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    run ${KPOD_BINARY} ${KPOD_OPTIONS} images --format json
    echo "$output" | python -m json.tool
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod images check name json output" {
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}
    run  ${KPOD_BINARY} ${KPOD_OPTIONS} images --format json
    [ "$status" -eq 0 ]
    name=$(echo $output | python -c 'import sys; import json; print(json.loads(sys.stdin.read())[0])["names"][0]')
    [ "$name" == "docker.io/library/${IMAGE}:latest" ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}
