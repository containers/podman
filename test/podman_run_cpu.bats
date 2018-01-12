#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "run cpu-period test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpu-period=5000 ${ALPINE} cat /sys/fs/cgroup/cpu/cpu.cfs_period_us | tr -d '\r'"
    echo $output
    [ "$status" -eq 0 ]
    [ "$output" = 5000 ]
}

@test "run cpu-quota test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpu-quota=5000 ${ALPINE} cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 5000 ]
}

@test "run cpus test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpus=0.5 ${ALPINE} cat /sys/fs/cgroup/cpu/cpu.cfs_period_us | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 100000 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpus=0.5 ${ALPINE} cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 50000 ]
}

@test "run cpu-shares test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpu-shares=2 ${ALPINE} cat /sys/fs/cgroup/cpu/cpu.shares | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 2 ]
}

@test "run cpuset-cpus test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpuset-cpus=0 ${ALPINE} cat /sys/fs/cgroup/cpuset/cpuset.cpus | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 0 ]
}

@test "run cpuset-mems test" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpuset-mems=0 ${ALPINE} cat /sys/fs/cgroup/cpuset/cpuset.mems | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = 0 ]
}

@test "run failure if cpus and cpu-period set together test" {
    # skip, error code incorrect with bash -c and will fail centos test without bash -c
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpu-period=5000 --cpus=0.5 ${ALPINE} /bin/bash
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "run failure if cpus and cpu-quota set together test" {
    # skip, error code incorrect with bash -c and will fail centos test without bash -c
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --cpu-quota=5000 --cpus=0.5 ${ALPINE} /bin/bash
    echo "$output"
    [ "$status" -ne 0 ]
}
