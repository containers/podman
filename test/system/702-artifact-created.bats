#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman artifact created date functionality
#

load helpers

# Create temporary artifact file for testing
function create_test_file() {
    local content="$(random_string 100)"
    local filename=$(random_string 12)
    local filepath="$PODMAN_TMPDIR/$filename.txt"
    echo "$content" > "$filepath"
    echo "$filepath"
}

function teardown() {
    run_podman artifact rm --all --ignore
    basic_teardown
}

@test "podman artifact inspect shows created date in RFC3339 format" {
    local testfile1=$(create_test_file)
    local artifact_name="localhost/test/created-test"
    local testfile2=$(create_test_file)

    # Record time before creation (in nanoseconds for comparison)
    local before_epoch=$(date +%s%N)

    # Create artifact
    run_podman artifact add $artifact_name "$testfile1"

    # Record time after creation
    local after_epoch=$(date +%s%N)

    # Inspect the artifact
    run_podman artifact inspect --format '{{index  .Manifest.Annotations "org.opencontainers.image.created" }}' $artifact_name
    local created_annotation="$output"
    assert "$created_annotation" != "" "Created annotation should not be empty"

    # Verify it's a valid RFC3339 timestamp by trying to parse it
    # Convert to epoch for comparison
    local created_epoch=$(date -d "$created_annotation" +%s%N 2>/dev/null)

    # Verify parsing succeeded
    assert "$?" -eq 0 "Created timestamp should be valid RFC3339 format"

    # Verify timestamp is within reasonable bounds
    assert "$created_epoch" -ge "$before_epoch" "Created time should be after before_epoch"
    assert "$created_epoch" -le "$after_epoch" "Created time should be before after_epoch"

    # Append to artifact
    run_podman artifact add --append $artifact_name "$testfile2"

    # Get the created timestamp after append
    run_podman artifact inspect --format '{{index  .Manifest.Annotations "org.opencontainers.image.created" }}\n{{len .Manifest.Layers}}' $artifact_name
    local current_created="${lines[0]}"
    local layer_count="${lines[1]}"

    # Verify the created timestamp is preserved
    assert "$current_created" = "$created_annotation" "Created timestamp should be preserved during append"

    # Verify we have 2 layers now
    assert "$layer_count" -eq 2 "Should have 2 layers after append"
}

# vim: filetype=sh
