#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function start_sleep_container () {
    pod_id=$(crioctl pod run --config "$TESTDATA"/sandbox_config.json)
    ctr_id=$(crioctl ctr create --config "$TESTDATA"/container_config_sleep.json --pod "$pod_id")
    crioctl ctr start --id "$ctr_id"
}

@test "kill a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} kill foobar
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "kill a running container by id" {
    skip "Test needs to be converted to kpod run"
    start_crio
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    ctr_id=$( start_sleep_container )
    crioctl ctr status --id "$ctr_id"
    ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    ${KPOD_BINARY} ${KPOD_OPTIONS} logs "$ctr_id"
    crioctl ctr status --id "$ctr_id"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} kill "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kill a running container by id with TERM" {
    skip "Test needs to be converted to kpod run"
    start_crio
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    ctr_id=$( start_sleep_container )
    crioctl ctr status --id "$ctr_id"
    ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    ${KPOD_BINARY} ${KPOD_OPTIONS} logs "$ctr_id"
    crioctl ctr status --id "$ctr_id"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} kill -s TERM "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kill a running container by name" {
    skip "Test needs to be converted to kpod run"
    start_crio
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    ctr_id=$( start_sleep_container )
    crioctl ctr status --id "$ctr_id"
    ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    ${KPOD_BINARY} ${KPOD_OPTIONS} logs "$ctr_id"
    crioctl ctr status --id "$ctr_id"
    ${KPOD_BINARY} ${KPOD_OPTIONS} ps -a
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} kill "k8s_container999_podsandbox1_redhat.test.crio_redhat-test-crio_1"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "kill a running container by id with a bogus signal" {
    skip "Test needs to be converted to kpod run"
    start_crio
    ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    ctr_id=$( start_sleep_container )
    crioctl ctr status --id "$ctr_id"
    ${KPOD_BINARY} ${KPOD_OPTIONS} logs "$ctr_id"
    crioctl ctr status --id "$ctr_id"
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} kill -s foobar "$ctr_id"
    echo "$output"
    [ "$status" -ne 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
