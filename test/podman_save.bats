#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "podman save output flag" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save -o alpine.tar $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "podman save oci flag" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save -o alpine.tar --format oci-archive $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "podman save using stdout" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save $ALPINE > alpine.tar"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "podman save quiet flag" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save -q -o alpine.tar $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "podman save non-existent image" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save -o alpine.tar FOOBAR"
	echo "$output"
	[ "$status" -ne 0 ]
}

@test "podman save to directory wit oci format" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save --format oci-dir -o alp-dir $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -rf alp-dir
}

@test "podman save to directory wit v2s2 (docker) format" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} save --format docker-dir -o alp-dir $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -rf alp-dir
}
