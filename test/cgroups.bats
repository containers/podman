#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "pids limit" {
	if ! grep pids /proc/self/cgroup; then
		skip "pids cgroup controller is not mounted"
	fi
	PIDS_LIMIT=1234 start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	pids_limit_config=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin); obj["command"] = ["/bin/sleep", "600"]; json.dump(obj, sys.stdout)')
	echo "$pids_limit_config" > "$TESTDIR"/container_pids_limit.json
	run crioctl ctr create --config "$TESTDIR"/container_pids_limit.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" cat /sys/fs/cgroup/pids/pids.max
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "1234" ]]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}
