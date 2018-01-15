#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "podman import with source and reference" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $ALPINE sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -cp "${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} import container.tar imported-image"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} images"
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"imported-image"* ]]
    rm -f container.tar
}

@test "podman import without reference" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $ALPINE sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} import container.tar"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} images"
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"<none>"* ]]
    rm -f container.tar
}

@test "podman import with message flag" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $ALPINE sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} import --message 'importing container test message' container.tar imported-image"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} history imported-image"
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"importing container test message"* ]]
    rm -f container.tar
}

@test "podman import with change flag" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run -d $ALPINE sleep 60"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} export -o container.tar $ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} import --change 'CMD=/bin/bash' container.tar imported-image"
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect imported-image"
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"/bin/bash"* ]]
    rm -f container.tar
}
