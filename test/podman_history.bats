#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    prepare_network_conf
    copy_images
}

@test "podman history default" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history with Go template format" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history --format "{{.ID}} {{.Created}}" $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history human flag" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history --human=false $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history quiet flag" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history -q $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history no-trunc flag" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history --no-trunc $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history json flag" {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} history --format json $ALPINE | python -m json.tool"
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman history short options" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} history -qH $ALPINE
	echo "$output"
	[ "$status" -eq 0 ]
}
