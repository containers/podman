#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "podman commit default" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d --name my_ctr ${FEDORA_MINIMAL} sleep 6000
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} images | grep image-committed"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop my_ctr
}

@test "podman commit with message flag" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d --name my_ctr ${FEDORA_MINIMAL} sleep 6000
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit --message testing-commit my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect image-committed | grep testing-commit"
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop my_ctr
}

@test "podman commit with author flag" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d --name my_ctr ${FEDORA_MINIMAL} sleep 6000
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit --author author-name my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect image-committed | grep author-name"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop my_ctr
}

@test "podman commit with change flag" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d --name my_ctr ${FEDORA_MINIMAL} sleep 6000
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit --change LABEL=image=blue my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect image-committed | grep blue"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop my_ctr
}

@test "podman commit with pause flag" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d --name my_ctr ${FEDORA_MINIMAL} sleep 6000
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit --pause=false my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} images | grep image-committed"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} stop my_ctr
}

@test "podman commit non-running container" {
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --name my_ctr ${FEDORA_MINIMAL} ls
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} commit my_ctr image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} images | grep image-committed"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi image-committed
    echo "$output"
    [ "$status" -eq 0 ]
    ${PODMAN_BINARY} ${PODMAN_OPTIONS} rm my_ctr
}
