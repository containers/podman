#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

@test "kpod rename successful" {
    start_crio
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
    echo "$output"
    [ "$status" -eq 0 ]
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    pod_id="$output"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    ctr_id="$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} $KPOD_OPTIONS rename "$ctr_id" "$NEW_NAME"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} $KPOD_OPTIONS inspect "$ctr_id" --format {{.Name}}
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" == "$NEW_NAME" ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
