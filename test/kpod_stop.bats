#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

@test "stop a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop foobar
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stop a running container by id" {
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
    id="$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "$id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "stop a running container by name" {
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
    [ "$status" -eq 0 ]
    run crioctl ctr inspect --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_name=$(python -c 'import json; import sys; print json.load(sys.stdin)["crio_annotations"]["io.kubernetes.cri-o.Name"]' <<< "$output")
    echo container name is \""$ctr_name"\"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "$ctr_name"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}
