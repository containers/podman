#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod save output flag" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save oci flag" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save using stdout" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save $ALPINE > alpine.tar"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save quiet flag" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save -q -o alpine.tar $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save non-existent image" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar FOOBAR"
	echo "$output"
	[ "$status" -ne 0 ]
}

@test "kpod save to directory wit oci format" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save --format oci-dir -o alp-dir $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -rf alp-dir
}

@test "kpod save to directory wit v2s2 (docker) format" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} save --format docker-dir -o alp-dir $ALPINE"
	echo "$output"
	[ "$status" -eq 0 ]
	rm -rf alp-dir
}
