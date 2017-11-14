#!/usr/bin/env bats

load helpers

IMAGE="alpine:latest"

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "remove a stopped container" {
    skip "Test needs to be converted to kpod run"
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run crioctl ctr stop --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "refuse to remove a running container" {
    skip "Test needs to be converted to kpod run"
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm "$ctr_id"
    echo "$output"
    [ "$status" -ne 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "remove a created container" {
    skip "Test needs to be converted to kpod run"
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "remove a running container" {
    skip "Test needs to be converted to kpod run"
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rm -f "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}
