#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod save output flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save oci flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save using stdout" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save > alpine.tar $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save quiet flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -q -o alpine.tar $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod save non-existent image" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar FOOBAR
	echo "$output"
	[ "$status" -ne 0 ]
}
