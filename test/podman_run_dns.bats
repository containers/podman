#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "test addition of a search domain" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --dns-search=foobar.com ${ALPINE} cat /etc/resolv.conf | grep foo"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test addition of a bad dns server" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --dns="foo" ${ALPINE} ls
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "test addition of a dns server" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --dns='1.2.3.4' ${ALPINE} cat /etc/resolv.conf | grep '1.2.3.4'"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test addition of a dns option" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --dns-opt='debug' ${ALPINE} cat /etc/resolv.conf | grep 'options debug'"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "test addition of a bad add-host" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} create --add-host="foo:1.2" ${ALPINE} ls
    echo "$output"
    [ "$status" -ne 0 ]
}

@test "test addition of add-host" {
    run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} run --rm --add-host='foobar:1.1.1.1' ${ALPINE} cat /etc/hosts | grep 'foobar'"
    echo "$output"
    [ "$status" -eq 0 ]
}
