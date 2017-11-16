#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

@test "kpod export output flag" {
    skip "Test needs to be converted to kpod run bash -c"
    start_crio
    run bash -c crioctl pod run bash -c --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run bash -c crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}
