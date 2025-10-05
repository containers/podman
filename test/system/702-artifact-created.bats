#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman artifact created date functionality
#

load helpers

# Create temporary artifact file for testing
function create_test_file() {
    local content="$1"
    local filename=$(random_string 12)
    local filepath="$PODMAN_TMPDIR/$filename.txt"
    echo "$content" > "$filepath"
    echo "$filepath"
}

function setup() {
    basic_setup
    skip_if_remote "artifacts are not remote"
}

function teardown() {
    run_podman artifact rm --all --ignore || true
    basic_teardown
}

@test "podman artifact inspect shows created date in RFC3339 format" {
    local content="test content for created date"
    local testfile1=$(create_test_file "$content")
    local artifact_name="localhost/test/created-test"
    local content2="appended content"
    local testfile2=$(create_test_file "$content2")

    # Record time before creation (in seconds for comparison)
    local before_epoch=$(date +%s)

    # Create artifact
    run_podman artifact add $artifact_name "$testfile1"

    # Record time after creation (in seconds for comparison)
    local after_epoch=$(date +%s)
    after_epoch=$((after_epoch + 1))

    # Inspect the artifact
    run_podman artifact inspect $artifact_name
    local output="$output"

    # Parse the JSON output to get the created annotation
    local created_annotation
    created_annotation=$(echo "$output" | jq -r '.Manifest.annotations["org.opencontainers.image.created"]')

    # Verify created annotation exists and is not null
    assert "$created_annotation" != "null" "Should have org.opencontainers.image.created annotation"
    assert "$created_annotation" != "" "Created annotation should not be empty"

    # Verify it's a valid RFC3339 timestamp by trying to parse it
    # Convert to epoch for comparison
    local created_epoch
    created_epoch=$(date -d "$created_annotation" +%s 2>/dev/null)

    # Verify parsing succeeded
    assert "$?" -eq 0 "Created timestamp should be valid RFC3339 format"

    # Verify timestamp is within reasonable bounds
    assert "$created_epoch" -ge "$before_epoch" "Created time should be after before_epoch"
    assert "$created_epoch" -le "$after_epoch" "Created time should be before after_epoch"

    # Wait a bit to ensure timestamps would differ if created new
    sleep 1

    # Append to artifact
    run_podman artifact add --append $artifact_name "$testfile2"

    # Get the created timestamp after append
    run_podman artifact inspect $artifact_name
    local current_created
    current_created=$(echo "$output" | jq -r '.Manifest.annotations["org.opencontainers.image.created"]')

    # Verify the created timestamp is preserved
    assert "$current_created" = "$created_annotation" "Created timestamp should be preserved during append"

    # Verify we have 2 layers now
    local layer_count
    layer_count=$(echo "$output" | jq '.Manifest.layers | length')
    assert "$layer_count" -eq 2 "Should have 2 layers after append"

    # Clean up
    rm -f "$testfile1" "$testfile2"
}

# vim: filetype=sh
