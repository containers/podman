#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "ctr termination reason Completed" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config_selinux.json
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

	cleanup_ctrs
	cleanup_pods
	stop_crio
}
