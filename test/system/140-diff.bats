#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman diff
#

load helpers

@test "podman diff" {
    rand_file=$(random_string 10)
    run_podman run $IMAGE sh -c "touch /$rand_file;rm /etc/services"
    run_podman diff --format json -l

    # Expected results for each type of diff
    declare -A expect=(
        [added]="/$rand_file"
        [changed]="/etc"
        [deleted]="/etc/services"
    )

    for field in ${!expect[@]}; do
        result=$(jq -r -c ".${field}[]" <<<"$output")
        is "$result" "${expect[$field]}" "$field"
    done

    run_podman rm -l
}

# vim: filetype=sh
