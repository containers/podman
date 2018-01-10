#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

@test "podman search" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search alpine
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman search registry flag" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search --registry registry.fedoraproject.org fedora
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman search filter flag" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search --filter=is-official alpine
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman search format flag" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search --format "table {{.Index}} {{.Name}}" alpine
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman search no-trunc flag" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search --no-trunc alpine
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "podman search limit flag" {
    run ${PODMAN_BINARY} ${PODMAN_OPTIONS} search --limit 3 alpine
    echo "$output"
    [ "$status" -eq 0 ]
}