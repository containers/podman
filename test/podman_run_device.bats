#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "run baddevice test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -q --device /dev/baddevice ${ALPINE}  ls /dev/kmsg
    echo $output
    [ "$status" -ne 0 ]
}

@test "run device test" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} run -q --device /dev/kmsg ${ALPINE}  ls --color=never /dev/kmsg
    echo "$output"
    [ "$status" -eq 0 ]
    device=$(echo $output | tr -d '\r')
    echo "<$device>"
    [ "$device" = "/dev/kmsg" ]
}
