#!/usr/bin/env bats

load helpers

ALPINE="docker.io/library/alpine:latest"

@test "run a container based on local image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    echo "$output"
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run docker.io/library/busybox:latest ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "run a container based on a remote image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
}
