#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

@test "kpod export output flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}
