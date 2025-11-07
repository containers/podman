#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman artifact functionality
#

load helpers

# FIXME #27264: Artifact store does not seem to work properly with concurrent access. Do not the ci:parallel tags here!

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

@test "podman artifact add --replace basic functionality" {
    local artifact_name="localhost/test/replace-artifact"
    local file1 file2

    file1=$(create_test_file 1024)
    file2=$(create_test_file 2048)

    # Add initial artifact
    run_podman artifact add "$artifact_name" "$file1"
    local first_digest="$output"

    # Verify initial artifact exists
    run_podman artifact inspect "$artifact_name"

    # Replace with different file
    run_podman artifact add --replace "$artifact_name" "$file2"
    local second_digest="$output"

    # Verify artifact was replaced (different digest)
    assert "$first_digest" != "$second_digest" "Replace should create different digest"

    # Verify artifact still exists and is accessible
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
    rm -f "$file1" "$file2"
}

@test "podman artifact add --replace nonexistent artifact" {
    local artifact_name="localhost/test/nonexistent-artifact"
    local file1

    file1=$(create_test_file 1024)

    # Using --replace on nonexistent artifact should succeed
    run_podman artifact add --replace "$artifact_name" "$file1"

    # Verify artifact was created
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
    rm -f "$file1"
}

@test "podman artifact add --replace and --append conflict" {
    local artifact_name="localhost/test/conflict-artifact"
    local file1

    file1=$(create_test_file 1024)

    # Using --replace and --append together should fail
    run_podman 125 artifact add --replace --append "$artifact_name" "$file1"
    assert "$output" =~ "--append and --replace options cannot be used together"

    rm -f "$file1"
}

@test "podman artifact add --replace with existing artifact" {
    local artifact_name="localhost/test/existing-artifact"
    local file1 file2

    file1=$(create_test_file 512)
    file2=$(create_test_file 1024)

    # Create initial artifact
    run_podman artifact add "$artifact_name" "$file1"

    # Verify initial artifact exists
    run_podman artifact inspect "$artifact_name"

    # Adding same name without --replace should fail
    run_podman 125 artifact add "$artifact_name" "$file2"
    assert "$output" =~ "artifact already exists"

    # Replace should succeed
    run_podman artifact add --replace "$artifact_name" "$file2"

    # Verify artifact was replaced
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
    rm -f "$file1" "$file2"
}
