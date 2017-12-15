#!/usr/bin/env bats

load helpers

IMAGE="busybox"

function setup() {
    prepare_network_conf
}

function teardown() {
    cleanup_test
}

@test "podman images" {
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman images test valid json" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${IMAGE}
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} images --format json
    echo "$output" | python -m json.tool
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman images check name json output" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${IMAGE}
    run  ${PODMAN_BINARY} ${PODMAN_OPTIONS} images --format json
    [ "$status" -eq 0 ]
    name=$(echo $output | python -c 'import sys; import json; print(json.loads(sys.stdin.read())[0])["names"][0]')
    [ "$name" == "docker.io/library/${IMAGE}:latest" ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi ${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman images short options" {
    run  ${PODMAN_BINARY} ${PODMAN_OPTIONS} images -qn
    echo "$output"
    [ "$status" -eq 0 ]
}
