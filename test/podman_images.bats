#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}
@test "podman images" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman images test valid json" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} images --format json
    echo "$output" | python -m json.tool
    [ "$status" -eq 0 ]
}

@test "podman images check name json output" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi -fa
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull ${ALPINE}
    run  ${PODMAN_BINARY} ${PODMAN_OPTIONS} images --format json
    [ "$status" -eq 0 ]
    name=$(echo $output | python -c 'import sys; import json; print(json.loads(sys.stdin.read())[0])["names"][0]')
    [ "$name" == "docker.io/library/alpine:latest" ]
}

@test "podman images short options" {
    run  ${PODMAN_BINARY} ${PODMAN_OPTIONS} images -qn
    echo "$output"
    [ "$status" -eq 0 ]
}
