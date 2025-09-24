#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman artifact rm --ignore functionality
#

load helpers

function setup() {
    basic_setup
}

function teardown() {
    basic_teardown
}

# Helper function to create a test artifact file
create_test_file() {
    local size=${1:-1024}
    local filename=$(mktemp --tmpdir="${PODMAN_TMPDIR}" artifactfile.XXXXXX)
    dd if=/dev/urandom of="$filename" bs=1 count="$size" 2>/dev/null
    echo "$filename"
}

# bats test_tags=ci:parallel
@test "podman artifact rm --ignore nonexistent artifact" {
    local artifact_name="localhost/test/nonexistent-artifact"

    # Removing nonexistent artifact without --ignore should fail
    run_podman 125 artifact rm "$artifact_name"
    assert "$output" =~ "artifact does not exist"

    # Removing nonexistent artifact with --ignore should succeed
    run_podman artifact rm --ignore "$artifact_name"
    is "$output" "" "No output expected when ignoring nonexistent artifact"
}

# bats test_tags=ci:parallel
@test "podman artifact rm --ignore with existing artifact" {
    local artifact_name="localhost/test/existing-artifact"
    local file1

    file1=$(create_test_file 1024)

    # Add artifact
    run_podman artifact add "$artifact_name" "$file1"
    local digest="$output"

    # Remove with --ignore should work normally
    run_podman artifact rm --ignore "$artifact_name"
    assert "$output" =~ "$digest" "Should output digest of removed artifact"

    # Verify artifact was removed
    run_podman 125 artifact inspect "$artifact_name"
    assert "$output" =~ "artifact does not exist"

    rm -f "$file1"
}

# bats test_tags=ci:parallel
@test "podman artifact rm --ignore mixed artifacts" {
    local existing1="localhost/test/existing1"
    local existing2="localhost/test/existing2"
    local nonexistent="localhost/test/nonexistent"
    local file1 file2

    file1=$(create_test_file 512)
    file2=$(create_test_file 1024)

    # Add two artifacts
    run_podman artifact add "$existing1" "$file1"
    local digest1="$output"

    run_podman artifact add "$existing2" "$file2"
    local digest2="$output"

    # Try removing mix without --ignore should fail
    run_podman 125 artifact rm "$nonexistent" "$existing1" "$existing2"
    assert "$output" =~ "artifact does not exist"

    # Verify existing artifacts are still there after failure
    run_podman artifact inspect "$existing1"
    run_podman artifact inspect "$existing2"

    # Try removing mix with --ignore should succeed
    run_podman artifact rm --ignore "$existing1" "$nonexistent" "$existing2"

    # Output should contain digests of removed artifacts
    assert "$output" =~ "$digest1" "Should contain digest of first artifact"
    assert "$output" =~ "$digest2" "Should contain digest of second artifact"

    # Verify artifacts were removed
    run_podman 125 artifact inspect "$existing1"
    run_podman 125 artifact inspect "$existing2"

    rm -f "$file1" "$file2"
}

# bats test_tags=ci:parallel
@test "podman artifact rm --ignore with --all" {
    local artifact1="localhost/test/artifact1"
    local artifact2="localhost/test/artifact2"
    local file1 file2

    file1=$(create_test_file 512)
    file2=$(create_test_file 1024)

    # Add artifacts
    run_podman artifact add "$artifact1" "$file1"
    run_podman artifact add "$artifact2" "$file2"

    # Remove all with --ignore should work
    run_podman artifact rm --ignore --all

    # Verify artifacts were removed
    run_podman 125 artifact inspect "$artifact1"
    run_podman 125 artifact inspect "$artifact2"

    rm -f "$file1" "$file2"
}
