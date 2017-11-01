#!/usr/bin/env bats

load helpers

@test "crio commands" {
	run ${CRIO_BINARY} --config /dev/null config > /dev/null
	echo "$output"
	[ "$status" -eq 0 ]
	run ${CRIO_BINARY} badoption > /dev/null
	echo "$output"
	[ "$status" -ne 0 ]
}
