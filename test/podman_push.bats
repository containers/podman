#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "podman push to containers/storage" {
    skip "Issues with bash, skipping"
    echo # Push the image right back into storage: it now has two names.
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS --log-level=debug push $ALPINE containers-storage:busybox:test
    echo "$output"
    [ "$status" -eq 0 ]
    echo # Try to remove it using the first name.  Should be refused.
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS --log-level=debug rmi $ALPINE
    echo "$output"
    [ "$status" -ne 0 ]
    echo # Try to remove it using the second name.  Should also be refused.
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS --log-level=debug rmi busybox:test
    echo "$output"
    [ "$status" -ne 0 ]
    echo # Force removal despite having multiple names.  Should succeed.
    run ${PODMAN_BINARY} $PODMAN_OPTIONS --log-level=debug rmi -f busybox:test
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman push to directory" {
    mkdir /tmp/busybox
    run ${PODMAN_BINARY} $PODMAN_OPTIONS push $ALPINE dir:/tmp/busybox
    echo "$output"
    [ "$status" -eq 0 ]
    rm -rf /tmp/busybox
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman push to docker archive" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS push $ALPINE docker-archive:/tmp/busybox-archive:1.26
    echo "$output"
    echo "--->"
    [ "$status" -eq 0 ]
    rm /tmp/busybox-archive
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman push to oci-archive without compression" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS push $ALPINE oci-archive:/tmp/oci-busybox.tar:alpine
    echo "$output"
    [ "$status" -eq 0 ]
    rm -f /tmp/oci-busybox.tar
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman push without signatures" {
    mkdir /tmp/busybox
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS push --remove-signatures $ALPINE dir:/tmp/busybox
    echo "$output"
    [ "$status" -eq 0 ]
    rm -rf /tmp/busybox
    run bash -c ${PODMAN_BINARY} $PODMAN_OPTIONS rmi $ALPINE
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman push without transport" {
    run ${PODMAN_BINARY} $PODMAN_OPTIONS pull "$ALPINE"
    echo "$output"
    [ "$status" -eq 0 ]
    # TODO: The following should fail until a registry is running in Travis CI.
    run ${PODMAN_BINARY} $PODMAN_OPTIONS push "$ALPINE" localhost:5000/my-alpine
    echo "$output"
    [ "$status" -ne 0 ]
    run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi "$ALPINE"
    echo "$output"
}

@test "push with manifest type conversion" {
    run bash -c "${PODMAN_BINARY} $PODMAN_OPTIONS push --format oci "${BB}" dir:my-dir"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "grep "application/vnd.oci.image.config.v1+json" my-dir/manifest.json"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} $PODMAN_OPTIONS push --compress --format v2s2 "${BB}" dir:my-dir"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "grep "application/vnd.docker.distribution.manifest.v2+json" my-dir/manifest.json"
    echo "$output"
    [ "$status" -eq 0 ]
    rm -rf my-dir
}
