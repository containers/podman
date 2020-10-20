#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman events functionality
#

load helpers

@test "events with a filter by label" {
    skip_if_remote "FIXME: -remote does not include labels in event output"
    cname=test-$(random_string 30 | tr A-Z a-z)
    labelname=$(random_string 10)
    labelvalue=$(random_string 15)

    run_podman run --label $labelname=$labelvalue --name $cname --rm $IMAGE ls

    expect=".* container start [0-9a-f]\+ (image=$IMAGE, name=$cname,.* ${labelname}=${labelvalue}"
    run_podman events --filter type=container --filter container=$cname --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering by container name and label"

    # Same thing, but without the container-name filter
    run_podman events --filter type=container --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering just by label"

    # Now filter just by container name, no label
    run_podman events --filter type=container --filter container=$cname --filter event=start --stream=false
    is "$output" "$expect" "filtering just by label"
}
