#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

@test "kpod version test" {
	run ${KPOD_BINARY} version
	echo "$output"
	[ "$status" -eq 0 ]
}
