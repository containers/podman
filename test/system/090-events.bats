#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman events functionality
#

load helpers

@test "events with a filter by label" {
    skip_if_remote "Need to talk to Ed on why this is failing on remote"
    rand=$(random_string 30)
    run_podman 0  run --label foo=bar --name test-$rand --rm $IMAGE ls
    run_podman 0 events --filter type=container --filter container=test-$rand --filter label=foo=bar --filter event=start --stream=false
    is "$output" ".*foo=bar" "check for label event on container with label"
}
