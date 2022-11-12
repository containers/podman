#!/usr/bin/env bats

load helpers

# Regression test for #8931
@test "podman images - bare manifest list" {
    # Create an empty manifest list and list images.

    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=$output

    run_podman manifest create test:1.0
    mid=$output
    run_podman manifest inspect --verbose $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman manifest inspect -v $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman images --format '{{.ID}}' --no-trunc
    is "$output" ".*sha256:$iid" "Original image ID still shown in podman-images output"
    run_podman rmi test:1.0
}

# vim: filetype=sh
