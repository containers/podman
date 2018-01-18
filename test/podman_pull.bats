#!/usr/bin/env bats

load helpers

function teardown() {
  cleanup_test
}

@test "podman pull from docker with tag" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull debian:6.0.10
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi debian:6.0.10
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from docker without tag" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS pull debian
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi debian
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from a non-docker registry with tag" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS pull registry.fedoraproject.org/fedora:rawhide
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi registry.fedoraproject.org/fedora:rawhide
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from a non-docker registry without tag" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS pull registry.fedoraproject.org/fedora
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi registry.fedoraproject.org/fedora
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull using digest" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS pull alpine@sha256:1072e499f3f655a032e88542330cf75b02e7bdf673278f701d7ba61629ee3ebe
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi alpine:latest
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from a non existent image" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS pull umohnani/get-started
	echo "$output"
	[ "$status" -ne 0 ]
}

@test "podman pull from docker with shortname" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull debian
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi docker.io/debian:latest
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from docker with shortname and tag" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull debian:6.0.10
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi docker.io/debian:6.0.10
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman pull from docker-archive" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} save -o alp.tar alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull docker-archive:alp.tar
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f alp.tar
}

@test "podman pull from oci-archive" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} save --format oci-archive -o oci-alp.tar alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull oci-archive:oci-alp.tar
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
	rm -f oci-alp.tar
}

@test "podman pull from local directory" {
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run mkdir test_pull_dir
	echo "$output"
    [ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} push alpine dir:test_pull_dir
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} pull dir:test_pull_dir
	echo "$output"
	[ "$status" -eq 0 ]
	run ${PODMAN_BINARY} ${PODMAN_OPTIONS} rmi test_pull_dir
	echo "$output"
	[ "$status" -eq 0 ]
	rm -rf test_pull_dir
}
