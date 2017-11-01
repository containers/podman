#!/usr/bin/env bats

load helpers

IMAGE="alpine:latest"

function teardown() {
    cleanup_test
}

@test "kpod history default" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history with Go template format" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --format "{{.ID}} {{.Created}}" $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history human flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --human=false $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history quiet flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history -q $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history no-trunc flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --no-trunc $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history json flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} history --format json $IMAGE | python -m json.tool"
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
}
