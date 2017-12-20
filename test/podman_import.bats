#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "podman import with source and reference" {
    skip "Test needs to be converted to podman run bash -c"
    start_crio
    run bash -c crioctl pod run bash -c --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run bash -c crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} import container.tar imported-image
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
    images="$output"
    run bash -c grep "imported-image" <<< "$images"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}

@test "podman import without reference" {
    skip "Test needs to be converted to podman run bash -c"
    start_crio
    run bash -c crioctl pod run bash -c --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run bash -c crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} import container.tar
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} images
    echo "$output"
    [ "$status" -eq 0 ]
    images="$output"
    run bash -c grep "<none>" <<< "$images"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}

@test "podman import with message flag" {
    skip "Test needs to be converted to podman run bash -c"
    start_crio
    run bash -c crioctl pod run bash -c --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run bash -c crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} import --message "importing container test message" container.tar imported-image
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} history imported-image
    echo "$output"
    [ "$status" -eq 0 ]
    history="$output"
    run bash -c grep "importing container test message" <<< "$history"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}

@test "podman import with change flag" {
    skip "Test needs to be converted to podman run bash -c"
    start_crio
    run bash -c crioctl pod run bash -c --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run bash -c crioctl image pull "$IMAGE"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} import --change "CMD=/bin/bash" container.tar imported-image
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect imported-image
    echo "$output"
    [ "$status" -eq 0 ]
    inspect="$output"
    run bash -c grep "/bin/bash" <<< "$inspect"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
    rm -f container.tar
}
