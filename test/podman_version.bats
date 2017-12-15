#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

@test "podman version test" {
	run bash -c "${PODMAN_BINARY} version"
	echo "$output"
	[ "$status" -eq 0 ]
}
