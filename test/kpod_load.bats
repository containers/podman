#!/usr/bin/env bats

load helpers

function setup() {
    copy_images
}

function teardown() {
    cleanup_test
}
@test "kpod load input flag" {
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod load oci-archive image" {
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $IMAGE
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod load oci-archive image with signature-policy" {
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $IMAGE
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
	cp /etc/containers/policy.json /tmp
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} load --signature-policy /tmp/policy.json -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f /tmp/policy.json
	rm -f alpine.tar
}

@test "kpod load using quiet flag" {
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} load -q -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
}

@test "kpod load non-existent file" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -ne 0 ]
}
