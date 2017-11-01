#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "crioctl runtimeversion" {
	start_crio
	run crioctl runtimeversion
	echo "$output"
	[ "$status" -eq 0 ]
	stop_crio
}
