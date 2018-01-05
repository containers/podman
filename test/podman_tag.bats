#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "podman tag with shortname:latest" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} tag ${ALPINE} foobar:latest"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect foobar:latest"
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi --force foobar:latest"
	[ "$status" -eq 0 ]
}

@test "podman tag with shortname" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} tag ${ALPINE} foobar"
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect foobar:latest"
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi --force foobar:latest"
	[ "$status" -eq 0 ]
}

@test "podman tag with shortname:tag" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} tag ${ALPINE} foobar:v"
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} inspect foobar:v"
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi --force foobar:v"
	[ "$status" -eq 0 ]
}
