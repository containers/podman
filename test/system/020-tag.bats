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

    # The order is intentionally wrong here to check the sorting
    # https://github.com/containers/podman/issues/23803
    local image1="registry.com/image:1"
    run_podman tag $IMAGE $image1
    local image3="registry.com/image:3"
    run_podman tag $IMAGE $image3
    local image2="registry.com/image:2"
    run_podman tag $IMAGE $image2

    local imageA="registry.com/aaa:a"
    run_podman tag $IMAGE $imageA

    local nl="
"
    run_podman images --format '{{.Repository}}:{{.Tag}}' --sort repository
    assert "$output" =~ "$imageA${nl}$image1${nl}$image2${nl}$image3" "images are sorted by repository and tag"

    run_podman untag $IMAGE $imageA $image1 $image2 $image3

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
