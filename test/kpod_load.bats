#!/usr/bin/env bats

load helpers

IMAGE="alpine:latest"

function teardown() {
    cleanup_test
}

@test "kpod load input flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod load oci-archive image" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
}

@test "kpod load oci-archive image with signature-policy" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar --format oci-archive $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
	cp /etc/containers/policy.json /tmp
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load --signature-policy /tmp/policy.json -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f /tmp/policy.json
	rm -f alpine.tar
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	[ "$status" -eq 0 ]
}

@test "kpod load using quiet flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} save -o alpine.tar $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load -q -i alpine.tar
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alpine.tar
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi $IMAGE
	[ "$status" -eq 0 ]
}

@test "kpod load non-existent file" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} load -i alpine.tar
	echo "$output"
	[ "$status" -ne 0 ]
}
