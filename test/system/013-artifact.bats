#!/usr/bin/env bats   -*- bats -*-
#
# Tests for `podman artifact` commands
#

load helpers

function setup() {
    TEST_DIR="$(mktemp -d)"
    ARTIFACT_FILE="$(mktemp -p $TEST_DIR XXXXXX)"
    ARTIFACT_NAME="artifact-$(safename)"
}

function teardown() {
    run_podman rm "$ARTIFACT_NAME"
    run_podman artifact ls -n --format '{{.Repository}}' |\
    while read artifact ; do
        run_podman '?' artifact rm $artifact
    done
    rm -rf "$TEST_DIR"
    basic_teardown
}

@test "podman artifact add - add an OCI artifact" {
    run_podman artifact add "$ARTIFACT_NAME" "$ARTIFACT_FILE"
    assert "$status" -eq 0 "podman artifact add should succeed"
    assert "$output" =~ "^[0-9a-f]{64}\$" "Expected hexadecimal ID"
    run_podman 125 artifact add "$ARTIFACT_NAME" "$ARTIFACT_FILE"
}

@test "podman artifact ls - list available artifacts" {
    run_podman artifact add "$ARTIFACT_NAME" "$ARTIFACT_FILE"
    run_podman artifact ls
    assert "$status" -eq 0 "podman artifact ls should succeed"
    assert "$output" =~ "$ARTIFACT_NAME" "Expected artifact name in output"
    run_podman 125 artifact inspect notexists
}

@test "podman artifact inspect - get details of an artifact" {
    run_podman artifact add "$ARTIFACT_NAME" "$ARTIFACT_FILE"
    run_podman artifact inspect "$ARTIFACT_NAME"
    assert "$status" -eq 0 "podman artifact inspect should succeed"
    assert "$output" =~ "$ARTIFACT_NAME" "Expected artifact name in output"
    run_podman 125 artifact inspect notexists
}

@test "podman artifact rm - remove an artifact" {
    run_podman artifact add "$ARTIFACT_NAME" "$ARTIFACT_FILE"
    run_podman artifact rm "$ARTIFACT_NAME"
    assert "$status" -eq 0 "podman artifact rm should succeed"
    assert "$output" =~ "^[0-9a-f]{64}\$" "Expected hexadecimal ID"
    run_podman 125 artifact rm "$ARTIFACT_NAME"
}

## vim: filetype=sh
