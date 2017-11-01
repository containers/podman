#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

# PR#59
@test "pod release name on remove" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	id="$output"
	run crioctl pod stop --id "$id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	id="$output"
	run crioctl pod stop --id "$id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "pod remove" {
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

@test "pod stop ignores not found sandboxes" {
	start_crio

	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "pod list filtering" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json -name pod1 --label "a=b" --label "c=d" --label "e=f"
	echo "$output"
	[ "$status" -eq 0 ]
	pod1_id="$output"
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json -name pod2 --label "a=b" --label "c=d"
	echo "$output"
	[ "$status" -eq 0 ]
	pod2_id="$output"
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json -name pod3 --label "a=b"
	echo "$output"
	[ "$status" -eq 0 ]
	pod3_id="$output"
	run crioctl pod list --label "a=b" --label "c=d" --label "e=f" --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id"  ]]
	run crioctl pod list --label "g=h" --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	run crioctl pod list --label "a=b" --label "c=d" --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id"  ]]
	[[ "$output" =~ "$pod2_id"  ]]
	run crioctl pod list --label "a=b" --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id"  ]]
	[[ "$output" =~ "$pod2_id"  ]]
	[[ "$output" =~ "$pod3_id"  ]]
	run crioctl pod list --id "$pod1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id"  ]]
	# filter by truncated id should work as well
	run crioctl pod list --id "${pod1_id:0:4}"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id" ]]
	run crioctl pod list --id "$pod2_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod2_id"  ]]
	run crioctl pod list --id "$pod3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod3_id"  ]]
	run crioctl pod list --id "$pod1_id" --label "a=b"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod1_id"  ]]
	run crioctl pod list --id "$pod2_id" --label "a=b"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod2_id"  ]]
	run crioctl pod list --id "$pod3_id" --label "a=b"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$pod3_id"  ]]
	run crioctl pod list --id "$pod3_id" --label "c=d"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	run crioctl pod stop --id "$pod1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod stop --id "$pod2_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod2_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod stop --id "$pod3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_pods
	stop_crio
}

@test "pod metadata in list & status" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crioctl pod list --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	# TODO: expected value should not hard coded here
	[[ "$output" =~ "Name: podsandbox1" ]]
	[[ "$output" =~ "UID: redhat-test-crio" ]]
	[[ "$output" =~ "Namespace: redhat.test.crio" ]]
	[[ "$output" =~ "Attempt: 1" ]]

	run crioctl pod status --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	# TODO: expected value should not hard coded here
	[[ "$output" =~ "Name: podsandbox1" ]]
	[[ "$output" =~ "UID: redhat-test-crio" ]]
	[[ "$output" =~ "Namespace: redhat.test.crio" ]]
	[[ "$output" =~ "Attempt: 1" ]]

	cleanup_pods
	stop_crio
}

@test "pass pod sysctls to runtime" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crioctl ctr create --pod "$pod_id" --config "$TESTDATA"/container_redis.json
	echo "$output"
	[ "$status" -eq 0 ]
	container_id="$output"

	run crioctl ctr start --id "$container_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl ctr execsync --id "$container_id" sysctl kernel.shm_rmid_forced
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "kernel.shm_rmid_forced = 1" ]]

	run crioctl ctr execsync --id "$container_id" sysctl kernel.msgmax
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "kernel.msgmax = 8192" ]]

	run crioctl ctr execsync --id "$container_id" sysctl net.ipv4.ip_local_port_range
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "net.ipv4.ip_local_port_range = 1024	65000" ]]

	cleanup_pods
	stop_crio
}

@test "pod stop idempotent" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "pod remove idempotent" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "pod stop idempotent with ctrs already stopped" {
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
	[ "$status" -eq 0 ]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "restart crio and still get pod status" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	restart_crio
	run crioctl pod status --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "invalid systemd cgroup_parent fail" {
	if [[ "$CGROUP_MANAGER" != "systemd" ]]; then
		skip "need systemd cgroup manager"
	fi

	wrong_cgroup_parent_config=$(cat "$TESTDATA"/sandbox_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["linux"]["cgroup_parent"] = "podsandbox1.slice:container:infra"; json.dump(obj, sys.stdout)')
	echo "$wrong_cgroup_parent_config" > "$TESTDIR"/sandbox_wrong_cgroup_parent.json

	start_crio
	run crioctl pod run --config "$TESTDIR"/sandbox_wrong_cgroup_parent.json
	echo "$output"
	[ "$status" -eq 1 ]

	stop_crio
}

@test "systemd cgroup_parent correctly set" {
	if [[ "$CGROUP_MANAGER" != "systemd" ]]; then
		skip "need systemd cgroup manager"
	fi

	cgroup_parent_config=$(cat "$TESTDATA"/sandbox_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["linux"]["cgroup_parent"] = "/Burstable/pod_integration_tests-123"; json.dump(obj, sys.stdout)')
	echo "$cgroup_parent_config" > "$TESTDIR"/sandbox_systemd_cgroup_parent.json

	start_crio
	run crioctl pod run --config "$TESTDIR"/sandbox_systemd_cgroup_parent.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run systemctl list-units --type=slice
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Burstable-pod_integration_tests_123.slice" ]]

	cleanup_pods
	stop_crio
}
