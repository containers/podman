#!/usr/bin/env bats

load helpers

IMAGE1="docker.io/library/alpine:latest"
IMAGE2="docker.io/library/busybox:latest"
IMAGE3="docker.io/library/busybox:glibc"

function teardown() {
  cleanup_test
}

function pullImages() {
  ${KPOD_BINARY} $KPOD_OPTIONS pull $IMAGE1
  ${KPOD_BINARY} $KPOD_OPTIONS pull $IMAGE2
  ${KPOD_BINARY} $KPOD_OPTIONS pull $IMAGE3
}

@test "kpod rmi bogus image" {
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi debian:6.0.10
	echo "$output"
	[ "$status" -eq 1 ]
}

@test "kpod rmi image with fq name" {
	pullImages
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi $IMAGE1
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod rmi image with short name" {
	pullImages
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi alpine
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod rmi all images" {
	pullImages
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi -a
	echo "$output"
	[ "$status" -eq 0 ]
}

@test "kpod rmi all images forceably" {
	pullImages
	${KPOD_BINARY} $KPOD_OPTIONS create ${IMAGE1} ls
	run ${KPOD_BINARY} $KPOD_OPTIONS rmi -a -f
	echo "$output"
	[ "$status" -eq 0 ]
}
