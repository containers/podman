#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
	cleanup_test
}

@test "bind secrets mounts to container" {
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
    run crioctl ctr execsync --id "$ctr_id" cat /proc/mounts
    echo "$output"
    [ "$status" -eq 0 ]
    mount_info="$output"
    run grep /container/path1 <<< "$mount_info"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "default mounts correctly sorted with other mounts" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    host_path="$TESTDIR"/clash
    mkdir "$host_path"
    echo "clashing..." > "$host_path"/clashing.txt
    sed -e "s,%HPATH%,$host_path,g" "$TESTDATA"/container_redis_default_mounts.json > "$TESTDIR"/defmounts_pre.json
    sed -e 's,%CPATH%,\/container\/path1\/clash,g' "$TESTDIR"/defmounts_pre.json > "$TESTDIR"/defmounts.json
    run crioctl ctr create --config "$TESTDIR"/defmounts.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr execsync --id "$ctr_id" ls -la /container/path1/clash
    echo "$output"
    [ "$status" -eq 0 ]
    run crioctl ctr execsync --id "$ctr_id" cat /container/path1/clash/clashing.txt
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "clashing..." ]]
    run crioctl ctr execsync --id "$ctr_id" ls -la /container/path1
    echo "$output"
    [ "$status" -eq 0 ]
    run crioctl ctr execsync --id "$ctr_id" cat /container/path1/test.txt
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Testing secrets mounts!" ]]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
