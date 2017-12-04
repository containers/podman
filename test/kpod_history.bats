#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "kpod history default" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history with Go template format" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --format "{{.ID}} {{.Created}}" $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history human flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --human=false $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history quiet flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history -q $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history no-trunc flag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history --no-trunc $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history json flag" {
	run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} history --format json $ALPINE | python -m json.tool"
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod history short options" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} history -qH $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}
