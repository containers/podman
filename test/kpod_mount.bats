#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

@test "mount" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} mount $ctr_id
    echo "$output"
    echo ${KPOD_BINARY} ${KPOD_OPTIONS} mount $ctr_id
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount --notruncate | grep $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unmount $ctr_id
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} mount $ctr_id
    echo "$output"
    [ "$status" -eq 0 ]
    root="$output"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} mount --format=json | python -m json.tool | grep $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    touch $root/foobar
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unmount $ctr_id
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
