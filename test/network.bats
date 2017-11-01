#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_ctrs
	cleanup_pods
	stop_crio
	rm -f /var/lib/cni/networks/crionet_test_args/*
	chmod 0755 $CONMON_BINARY
	cleanup_test
}

@test "ensure correct hostname" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl ctr execsync --id "$ctr_id" sh -c "hostname"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "crioctl_host" ]]
	run crioctl ctr execsync --id "$ctr_id" sh -c "echo \$HOSTNAME"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "crioctl_host" ]]
	run crioctl ctr execsync --id "$ctr_id" sh -c "cat /etc/hostname"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "crioctl_host" ]]
}

@test "ensure correct hostname for hostnetwork:true" {
	start_crio
	hostnetworkconfig=$(cat "$TESTDATA"/sandbox_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["linux"]["security_context"]["namespace_options"]["host_network"] = True; obj["annotations"] = {}; obj["hostname"] = ""; json.dump(obj, sys.stdout)')
	echo "$hostnetworkconfig" > "$TESTDIR"/sandbox_hostnetwork_config.json
	run crioctl pod run --config "$TESTDIR"/sandbox_hostnetwork_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl ctr execsync --id "$ctr_id" sh -c "hostname"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$HOSTNAME" ]]
	run crioctl ctr execsync --id "$ctr_id" sh -c "echo \$HOSTNAME"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$HOSTNAME" ]]
	run crioctl ctr execsync --id "$ctr_id" sh -c "cat /etc/hostname"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$HOSTNAME" ]]
}

@test "Check for valid pod netns CIDR" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr_id="$output"

	check_pod_cidr $ctr_id
}

@test "Ping pod from the host" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr_id="$output"

	ping_pod $ctr_id
}

@test "Ping pod from another pod" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod1_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod1_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr1_id="$output"

	temp_sandbox_conf cni_test

	run crioctl pod run --config "$TESTDIR"/sandbox_config_cni_test.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod2_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod2_id"
	echo "$output"
	[ "$status" -eq 0  ]
	ctr2_id="$output"

	ping_pod_from_pod $ctr1_id $ctr2_id

	ping_pod_from_pod $ctr2_id $ctr1_id
}

@test "Ensure correct CNI plugin namespace/name/container-id arguments" {
	start_crio "" "" "" "prepare_plugin_test_args_network_conf"
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	[ "$status" -eq 0 ]

	. /tmp/plugin_test_args.out

	[ "$FOUND_CNI_CONTAINERID" != "redhat.test.crio" ]
	[ "$FOUND_CNI_CONTAINERID" != "podsandbox1" ]
	[ "$FOUND_K8S_POD_NAMESPACE" = "redhat.test.crio" ]
	[ "$FOUND_K8S_POD_NAME" = "podsandbox1" ]

	rm -rf /tmp/plugin_test_args.out
}

@test "Connect to pod hostport from the host" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config_hostport.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	get_host_ip
	echo $host_ip

	run crioctl ctr create --config "$TESTDATA"/container_config_hostport.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run nc -w 5 $host_ip 4888 </dev/null
	echo "$output"
	[ "$output" = "crioctl_host" ]
	[ "$status" -eq 0 ]
	run crioctl ctr stop --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "Clean up network if pod sandbox fails" {
	start_crio "" "" "" "prepare_plugin_test_args_network_conf"

	# make conmon non-executable to cause the sandbox setup to fail after
	# networking has been configured
	chmod 0644 $CONMON_BINARY
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	chmod 0755 $CONMON_BINARY
	echo "$output"
	[ "$status" -ne 0 ]

	# ensure that the server cleaned up sandbox networking if the sandbox
	# failed after network setup
	rm -f /var/lib/cni/networks/crionet_test_args/last_reserved_ip
	num_allocated=$(ls /var/lib/cni/networks/crionet_test_args | wc -l)
	[[ "${num_allocated}" == "0" ]]
}
