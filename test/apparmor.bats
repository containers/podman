#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

# 1. test running with loading the default apparmor profile.
# test that we can run with the default apparmor profile which will not block touching a file in `.`
@test "load default apparmor profile and run a container with it" {
    # this test requires apparmor, so skip this test if apparmor is not enabled.
    enabled=$(is_apparmor_enabled)
    if [[ "$enabled" -eq 0 ]]; then
        skip "skip this test since apparmor is not enabled."
    fi

    start_crio

    sed -e 's/%VALUE%/,"container\.apparmor\.security\.beta\.kubernetes\.io\/testname1": "runtime\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/apparmor1.json

    run crioctl pod run --name apparmor1 --config "$TESTDIR"/apparmor1.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --name testname1 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr execsync --id "$ctr_id" touch test.txt
    echo "$output"
    [ "$status" -eq 0 ]


    cleanup_ctrs
    cleanup_pods
    stop_crio
}

# 2. test running with loading a specific apparmor profile as crio default apparmor profile.
# test that we can run with a specific apparmor profile which will block touching a file in `.` as crio default apparmor profile.
@test "load a specific apparmor profile as default apparmor and run a container with it" {
    # this test requires apparmor, so skip this test if apparmor is not enabled.
    enabled=$(is_apparmor_enabled)
    if [[ "$enabled" -eq 0 ]]; then
        skip "skip this test since apparmor is not enabled."
    fi

    load_apparmor_profile "$APPARMOR_TEST_PROFILE_PATH"
    start_crio "" "$APPARMOR_TEST_PROFILE_NAME"

    sed -e 's/%VALUE%/,"container\.apparmor\.security\.beta\.kubernetes\.io\/testname2": "apparmor-test-deny-write"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/apparmor2.json

    run crioctl pod run --name apparmor2 --config "$TESTDIR"/apparmor2.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --name testname2 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr execsync --id "$ctr_id" touch test.txt
    echo "$output"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "Permission denied" ]]

    cleanup_ctrs
    cleanup_pods
    stop_crio
    remove_apparmor_profile "$APPARMOR_TEST_PROFILE_PATH"
}

# 3. test running with loading a specific apparmor profile but not as crio default apparmor profile.
# test that we can run with a specific apparmor profile which will block touching a file in `.`
@test "load default apparmor profile and run a container with another apparmor profile" {
    # this test requires apparmor, so skip this test if apparmor is not enabled.
    enabled=$(is_apparmor_enabled)
    if [[ "$enabled" -eq 0 ]]; then
        skip "skip this test since apparmor is not enabled."
    fi

    load_apparmor_profile "$APPARMOR_TEST_PROFILE_PATH"
    start_crio

    sed -e 's/%VALUE%/,"container\.apparmor\.security\.beta\.kubernetes\.io\/testname3": "apparmor-test-deny-write"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/apparmor3.json

    run crioctl pod run --name apparmor3 --config "$TESTDIR"/apparmor3.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --name testname3 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr execsync --id "$ctr_id" touch test.txt
    echo "$output"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "Permission denied" ]]

    cleanup_ctrs
    cleanup_pods
    stop_crio
    remove_apparmor_profile "$APPARMOR_TEST_PROFILE_PATH"
}

# 4. test running with wrong apparmor profile name.
# test that we can will fail when running a ctr with rong apparmor profile name.
@test "run a container with wrong apparmor profile name" {
    # this test requires apparmor, so skip this test if apparmor is not enabled.
    enabled=$(is_apparmor_enabled)
    if [[ "$enabled" -eq 0 ]]; then
        skip "skip this test since apparmor is not enabled."
    fi

    start_crio

    sed -e 's/%VALUE%/,"container\.apparmor\.security\.beta\.kubernetes\.io\/testname4": "not-exists"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/apparmor4.json

    run crioctl pod run --name apparmor4 --config "$TESTDIR"/apparmor4.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --name testname4 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -ne 0 ]
    [[ "$output" =~ "Creating container failed" ]]


    cleanup_ctrs
    cleanup_pods
    stop_crio
}

# 5. test running with default apparmor profile unloaded.
# test that we can will fail when running a ctr with rong apparmor profile name.
@test "run a container after unloading default apparmor profile" {
    # this test requires apparmor, so skip this test if apparmor is not enabled.
    enabled=$(is_apparmor_enabled)
    if [[ "$enabled" -eq 0 ]]; then
        skip "skip this test since apparmor is not enabled."
    fi

    start_crio
    remove_apparmor_profile "$FAKE_CRIO_DEFAULT_PROFILE_PATH"

    sed -e 's/%VALUE%/,"container\.apparmor\.security\.beta\.kubernetes\.io\/testname5": "runtime\/default"/g' "$TESTDATA"/sandbox_config_seccomp.json > "$TESTDIR"/apparmor5.json

    run crioctl pod run --name apparmor5 --config "$TESTDIR"/apparmor5.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --name testname5 --config "$TESTDATA"/container_redis.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr execsync --id "$ctr_id" touch test.txt
    echo "$output"
    [ "$status" -eq 0 ]


    cleanup_ctrs
    cleanup_pods
    stop_crio
}
