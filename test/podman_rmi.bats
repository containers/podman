#!/usr/bin/env bats

load helpers

IMAGE1="docker.io/library/alpine:latest"
IMAGE2="docker.io/library/busybox:latest"
IMAGE3="docker.io/library/busybox:glibc"

function teardown() {
  cleanup_test
}

function pullImages() {
  ${PODMAN_BINARY} $PODMAN_OPTIONS pull $IMAGE1
  ${PODMAN_BINARY} $PODMAN_OPTIONS pull $IMAGE2
  ${PODMAN_BINARY} $PODMAN_OPTIONS pull $IMAGE3
}

@test "podman rmi bogus image" {
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi debian:6.0.10
	echo "$output"
	[ "$status" -eq 125 ]
}

@test "podman rmi image with fq name" {
	pullImages
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi $IMAGE1
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman rmi image with short name" {
	pullImages
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman rmi all images" {
	pullImages
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi -a
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "podman rmi all images forceably with short options" {
	pullImages
	${PODMAN_BINARY} $PODMAN_OPTIONS create ${IMAGE1} ls
	run ${PODMAN_BINARY} $PODMAN_OPTIONS rmi -af
	echo "$output"
	[ "$status" -eq 0 ]
}
