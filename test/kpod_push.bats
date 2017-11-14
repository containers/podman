#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod push to containers/storage" {
    skip "Issues with bash, skipping"
    echo # Push the image right back into storage: it now has two names.
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS --log-level=debug push $ALPINE containers-storage:busybox:test
    echo "$output"
    [ "$status" -eq 0 ]
    echo # Try to remove it using the first name.  Should be refused.
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS --log-level=debug rmi $ALPINE
    echo "$output"
    [ "$status" -ne 0 ]
    echo # Try to remove it using the second name.  Should also be refused.
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS --log-level=debug rmi busybox:test
    echo "$output"
    [ "$status" -ne 0 ]
    echo # Force removal despite having multiple names.  Should succeed.
    run ${KPOD_BINARY} $KPOD_OPTIONS --log-level=debug rmi -f busybox:test
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod push to directory" {
    mkdir /tmp/busybox
    run ${KPOD_BINARY} $KPOD_OPTIONS push $ALPINE dir:/tmp/busybox
    echo "$output"
    [ "$status" -eq 0 ]
    rm -rf /tmp/busybox
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod push to docker archive" {
    run ${KPOD_BINARY} $KPOD_OPTIONS push $ALPINE docker-archive:/tmp/busybox-archive:1.26
    echo "$output"
    echo "--->"
    [ "$status" -eq 0 ]
    rm /tmp/busybox-archive
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod push to oci-archive without compression" {
    run ${KPOD_BINARY} $KPOD_OPTIONS push $ALPINE oci-archive:/tmp/oci-busybox.tar:alpine
    echo "$output"
    [ "$status" -eq 0 ]
    rm -f /tmp/oci-busybox.tar
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "kpod push without signatures" {
    mkdir /tmp/busybox
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS push --remove-signatures $ALPINE dir:/tmp/busybox
    echo "$output"
    [ "$status" -eq 0 ]
    rm -rf /tmp/busybox
    run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}
