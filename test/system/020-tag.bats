#!/usr/bin/env bats

load helpers

# helper function for "podman tag/untag" test
function _tag_and_check() {
    local tag_as="$1"
    local check_as="$2"

    run_podman tag $IMAGE $tag_as
    run_podman image exists $check_as
    run_podman untag $IMAGE $check_as
    run_podman 1 image exists $check_as
}

@test "podman tag/untag" {
    # Test a fully-qualified image reference.
    _tag_and_check registry.com/image:latest registry.com/image:latest

    # Test a reference without tag and make sure ":latest" is appended.
    _tag_and_check registry.com/image registry.com/image:latest

    # Test a tagged short image and make sure "localhost/" is prepended.
    _tag_and_check image:latest localhost/image:latest

    # Test a short image without tag and make sure "localhost/" is
    # prepended and ":latest" is appended.
    _tag_and_check image localhost/image:latest

    # Test error case.
    run_podman 125 untag $IMAGE registry.com/foo:bar
    is "$output" "Error: registry.com/foo:bar: tag not known"
}

@test "podman untag all" {
    # First get the image ID
    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=$output

    # Add a couple of tags
    run_podman tag $IMAGE registry.com/1:latest registry.com/2:latest registry.com/3:latest

    # Untag with arguments to for all tags to be removed
    run_podman untag $iid

    # Now make sure all tags are removed
    run_podman image inspect $iid --format "{{.RepoTags}}"
    is "$output" "\[\]" "untag by ID leaves empty set of tags"

    # Restore image
    run_podman tag $iid $IMAGE
}

# vim: filetype=sh
