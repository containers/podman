#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

@test "pause a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause foobar
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "unpause a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause foobar
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "pause a created container by id" {
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
    ctr_id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "pause a running container by id" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause "$id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause "$id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "pause a running container by name" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause "k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause "k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "remove a paused container by id" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    id="$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause "$id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rm "$id"
    echo "$output"
    [ "$status" -eq 1 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rm --force "$id"
    echo "$output"
    [ "$status" -eq 1 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause "$id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} rm "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}

@test "stop a paused container created by id" {
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
    ctr_id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pause "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "$ctr_id"
    echo "$output"
    [ "$status" -eq 1 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} unpause "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a --filter id="$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_pods
    stop_crio
}
