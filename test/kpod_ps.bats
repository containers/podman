#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"
function setup() {
    copy_images
}


@test "kpod ps with no containers" {
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod ps default" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps all flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --all
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps size flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a -s
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --size
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps quiet flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a -q
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --quiet
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps latest flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --latest
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -l
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps last flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --last 2
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -n 2
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps no-trunc flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --no-trunc
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps namespace flag" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --ns
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps --all --namespace
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps namespace flag and format flag = json" {
    skip "Test needs to be converted to kpod run"
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
    skip "Test needs to be converted to kpod run"
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
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --format "table {{.ID}} {{.Image}} {{.Labels}}"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - ancestor" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter ancestor=${IMAGE}
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - id" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kpod ps filter flag - status" {
    skip "Test needs to be converted to kpod run"
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
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter status=running
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
