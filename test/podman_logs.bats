#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "display logs for container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} logs $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "tail three lines of logs for container" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} logs --tail 3 $ctr_id"
    echo "$output"
    lines=$(echo "$output" | wc -l)
    [ "$status" -eq 0 ]
    [[ $(wc -l < "$output" ) -le 3 ]]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "display logs for container since a given time" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} logs --since 2017-08-07T10:10:09.056611202-04:00 $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
}
