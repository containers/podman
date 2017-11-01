#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

@test "kpod ps with no containers" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps default" {
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
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps all flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps --all
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps size flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a -s
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --size
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps quiet flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a -q
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --quiet
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps latest flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps --latest
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -l
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps last flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps --last 2
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -n 2
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps no-trunc flag" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --no-trunc
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps namespace flag" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps --all --namespace
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps namespace flag and format flag = json" {
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
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps without namespace flag and format flag = json" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format json | python -m json.tool | grep namespace"
    echo "$output"
    [ "$status" -eq 1 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps format flag = go template" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format "table {{.ID}} {{.Image}} {{.Labels}}"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - ancestor" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter ancestor=${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - id" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - status" {
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
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter status=running
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
