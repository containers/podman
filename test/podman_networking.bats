#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "test network connection with default bridge" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait --latest
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test network connection with host" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt --network host ${ALPINE} wget www.yahoo.com
    echo "$output"
    [ "$status" -eq 0 ]
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} wait --latest
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "expose port 222" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt --expose 222-223 ${ALPINE} /bin/sh
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "iptables -t nat -L"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "iptables -t nat -L | grep 223"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "expose host port 80 to container port 8000" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -dt -p 80:8000 ${ALPINE} /bin/sh
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "iptables -t nat -L | grep 8000"
    echo "$output"
    [ "$status" -eq 0 ]
}
