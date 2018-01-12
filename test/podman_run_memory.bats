#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "run memory test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --memory=40m ${ALPINE}  cat /sys/fs/cgroup/memory/memory.limit_in_bytes | tr -d '\r'"
    echo $output
    [ "$status" -eq 0 ]
    [ "$output" = 41943040 ]
}

@test "run memory-reservation test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --memory-reservation=40m ${ALPINE} cat /sys/fs/cgroup/memory/memory.soft_limit_in_bytes | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 41943040 ]
}

@test "run memory-swappiness test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --memory-swappiness=15 ${ALPINE} cat /sys/fs/cgroup/memory/memory.swappiness | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 15 ]
}

@test "run kernel-memory test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --kernel-memory=40m ${ALPINE} cat /sys/fs/cgroup/memory/memory.kmem.limit_in_bytes | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 41943040 ]
}
