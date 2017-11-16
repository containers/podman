#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

@test "stats single output" {
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "stats does not output stopped container" {
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "stats outputs stopped container with all flag" {
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream --all
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "stats output only id" {
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} stats --no-stream --format {{.ID}} "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    # once ps is implemented, run ps -q and see if that equals the output from above
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "stats streaming output" {
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
    run timeout 5s bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} stats --all"
    echo "$output"
    [ "$status" -eq 124 ] #124 is the status set by timeout when it has to kill the command at the end of the given time
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
