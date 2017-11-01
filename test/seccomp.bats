#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

# 1. test running with ctr unconfined
# test that we can run with a syscall which would be otherwise blocked
@test "ctr seccomp profiles unconfined" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/testname": "unconfined"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp1.json
	run crioctl pod run --name seccomp1 --config "$TESTDIR"/seccomp1.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --name testname --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 2. test running with ctr runtime/default
# test that we cannot run with a syscall blocked by the default seccomp profile
@test "ctr seccomp profiles runtime/default" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/testname2": "runtime\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp2.json
	run crioctl pod run --name seccomp2 --config "$TESTDIR"/seccomp2.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --name testname2 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	[[ "$output" =~ "Operation not permitted" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 3. test running with ctr wrong profile name
@test "ctr seccomp profiles wrong profile name" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/testname3": "notgood"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp3.json
	run crioctl pod run --name seccomp3 --config "$TESTDIR"/seccomp3.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --name testname3 --config "$TESTDATA"/container_config.json --pod "$pod_id"
	echo "$output"
	[ "$status" -ne 0 ]
	[[ "$output" =~ "unknown seccomp profile option:" ]]
	[[ "$output" =~ "notgood" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# TODO(runcom): need https://issues.k8s.io/36997
# 4. test running with ctr localhost/profile_name
@test "ctr seccomp profiles localhost/profile_name" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	#sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	#sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	#sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	#start_crio "$TESTDIR"/seccomp_profile1.json

	skip "need https://issues.k8s.io/36997"
}

# 5. test running with unkwown ctr profile falls back to pod profile
# unknown ctr -> unconfined
# pod -> runtime/default
# result: fail chmod
@test "ctr seccomp profiles falls back to pod profile" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/redhat\.test\.crio-seccomp2-1-testname2-0-not-exists": "unconfined", "seccomp\.security\.alpha\.kubernetes\.io\/pod": "runtime\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp5.json
	run crioctl pod run --name seccomp5 --config "$TESTDIR"/seccomp5.json
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
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	[[ "$output" =~ "Operation not permitted" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 6. test running with unkwown ctr profile and no pod, falls back to unconfined
# unknown ctr -> runtime/default
# pod -> NO
# result: success, running unconfined
@test "ctr seccomp profiles falls back to unconfined" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/redhat\.test\.crio-seccomp6-1-testname6-0-not-exists": "runtime-default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp6.json
	run crioctl pod run --name seccomp6 --config "$TESTDIR"/seccomp6.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --name testname6 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 1. test running with pod unconfined
# test that we can run with a syscall which would be otherwise blocked
@test "pod seccomp profiles unconfined" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"seccomp\.security\.alpha\.kubernetes\.io\/pod": "unconfined"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp1.json
	run crioctl pod run --name seccomp1 --config "$TESTDIR"/seccomp1.json
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
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 2. test running with pod runtime/default
# test that we cannot run with a syscall blocked by the default seccomp profile
@test "pod seccomp profiles runtime/default" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"seccomp\.security\.alpha\.kubernetes\.io\/pod": "runtime\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp2.json
	run crioctl pod run --name seccomp2 --config "$TESTDIR"/seccomp2.json
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
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	[[ "$output" =~ "Operation not permitted" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# 3. test running with pod wrong profile name
@test "pod seccomp profiles wrong profile name" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	# 3. test running with pod wrong profile name
	sed -e 's/%VALUE%/,"seccomp\.security\.alpha\.kubernetes\.io\/pod": "notgood"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp3.json
	run crioctl pod run --name seccomp3 --config "$TESTDIR"/seccomp3.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
	echo "$output"
	[ "$status" -ne 0 ]
	[[ "$output" =~ "unknown seccomp profile option:" ]]
	[[ "$output" =~ "notgood" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

# TODO(runcom): need https://issues.k8s.io/36997
# 4. test running with pod localhost/profile_name
@test "pod seccomp profiles localhost/profile_name" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	#sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	#sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	#sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	#start_crio "$TESTDIR"/seccomp_profile1.json

	skip "need https://issues.k8s.io/36997"
}

# test running with ctr docker/default
# test that we cannot run with a syscall blocked by the default seccomp profile
@test "ctr seccomp profiles docker/default" {
	# this test requires seccomp, so skip this test if seccomp is not enabled.
	enabled=$(is_seccomp_enabled)
	if [[ "$enabled" -eq 0 ]]; then
		skip "skip this test since seccomp is not enabled."
	fi

	sed -e 's/"chmod",//' "$CRIO_ROOT"/cri-o/seccomp.json > "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmod",//' "$TESTDIR"/seccomp_profile1.json
	sed -i 's/"fchmodat",//g' "$TESTDIR"/seccomp_profile1.json

	start_crio "$TESTDIR"/seccomp_profile1.json

	sed -e 's/%VALUE%/,"container\.seccomp\.security\.alpha\.kubernetes\.io\/testname2": "docker\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/seccomp2.json
	run crioctl pod run --name seccomp2 --config "$TESTDIR"/seccomp2.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --name testname2 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" chmod 777 .
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	[[ "$output" =~ "Operation not permitted" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}
