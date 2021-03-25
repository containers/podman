#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman load
#

load helpers

# Custom helpers for this test only. These just save us having to duplicate
# the same thing four times (two tests, each with -i and stdin).
#
# initialize, read image ID and name
get_iid_and_name() {
    run_podman images -a --format '{{.ID}} {{.Repository}}:{{.Tag}}'
    read iid img_name < <(echo "$output")

    archive=$PODMAN_TMPDIR/myimage-$(random_string 8).tar
}

# Simple verification of image ID and name
verify_iid_and_name() {
    run_podman images -a --format '{{.ID}} {{.Repository}}:{{.Tag}}'
    read new_iid new_img_name < <(echo "$output")

    # Verify
    is "$new_iid"      "$iid" "Image ID of loaded image == original"
    is "$new_img_name" "$1"   "Name & tag of restored image"
}

@test "podman load invalid file" {
    # Regression test for #9672 to make sure invalid input yields errors.
    invalid=$PODMAN_TMPDIR/invalid
    echo "I am an invalid file and should cause a podman-load error" > $invalid
    run_podman 125 load -i $invalid
    # podman and podman-remote emit different messages; this is a common string
    is "$output" ".*payload does not match any of the supported image formats .*" \
       "load -i INVALID fails with expected diagnostic"
}

@test "podman save to pipe and load" {
    # Generate a random name and tag (must be lower-case)
    local random_name=x0$(random_string 12 | tr A-Z a-z)
    local random_tag=t0$(random_string 7 | tr A-Z a-z)
    local fqin=localhost/$random_name:$random_tag
    run_podman tag $IMAGE $fqin

    # Believe it or not, 'podman load' would barf if any path element
    # included a capital letter
    archive=$PODMAN_TMPDIR/MySubDirWithCaps/MyImage-$(random_string 8).tar
    mkdir -p $(dirname $archive)

    # We can't use run_podman because that uses the BATS 'run' function
    # which redirects stdout and stderr. Here we need to guarantee
    # that podman's stdout is a pipe, not any other form of redirection
    $PODMAN save --format oci-archive $fqin | cat >$archive
    if [ "$status" -ne 0 ]; then
        die "Command failed: podman save ... | cat"
    fi

    # Make sure we can reload it
    run_podman rmi $fqin
    run_podman load -i $archive

    # FIXME: cannot compare IID, see #7371, so we check only the tag
    run_podman images $fqin --format '{{.Repository}}:{{.Tag}}'
    is "$output" "$fqin" "image preserves name across save/load"

    # Load with a new tag
    local new_name=x1$(random_string 14 | tr A-Z a-z)
    local new_tag=t1$(random_string 6 | tr A-Z a-z)
    run_podman rmi $fqin

    run_podman load -i $archive
    run_podman images --format '{{.Repository}}:{{.Tag}}' --sort tag
    is "${lines[0]}" "$IMAGE"     "image is preserved"
    is "${lines[1]}" "$fqin"      "image is reloaded with old fqin"

    # Clean up
    run_podman rmi $fqin
}


@test "podman load - by image ID" {
    # FIXME: how to build a simple archive instead?
    get_iid_and_name

    # Save image by ID, and remove it.
    run_podman save $iid -o $archive
    run_podman rmi $iid

    # Load using -i; IID should be preserved, but name is not.
    run_podman load -i $archive
    verify_iid_and_name "<none>:<none>"

    # Same as above, using stdin
    run_podman rmi $iid
    run_podman load < $archive
    verify_iid_and_name "<none>:<none>"

    # Same as above, using stdin but with `podman image load`
    run_podman rmi $iid
    run_podman image load < $archive
    verify_iid_and_name "<none>:<none>"

    # Cleanup: since load-by-iid doesn't preserve name, re-tag it;
    # otherwise our global teardown will rmi and re-pull our standard image.
    run_podman tag $iid $img_name
}

@test "podman load - by image name" {
    get_iid_and_name
    run_podman save $img_name -o $archive
    run_podman rmi $iid

    # Load using -i; this time the image should be tagged.
    run_podman load -i $archive
    verify_iid_and_name $img_name
    run_podman rmi $iid

    # Also make sure that `image load` behaves the same.
    run_podman image load -i $archive
    verify_iid_and_name $img_name
    run_podman rmi $iid

    # Same as above, using stdin
    run_podman load < $archive
    verify_iid_and_name $img_name
}

@test "podman load - redirect corrupt payload" {
    run_podman 125 load <<< "Danger, Will Robinson!! This is a corrupt tarball!"
    is "$output" \
        ".*payload does not match any of the supported image formats .*" \
        "Diagnostic from 'podman load' unknown/corrupt payload"
}

@test "podman load - multi-image archive" {
    img1="quay.io/libpod/testimage:00000000"
    img2="quay.io/libpod/testimage:20200902"
    archive=$PODMAN_TMPDIR/myimage-$(random_string 8).tar

    run_podman pull $img1
    run_podman pull $img2

    run_podman save -m -o $archive $img1 $img2
    run_podman rmi -f $img1 $img2
    run_podman load -i $archive

    run_podman image exists $img1
    run_podman image exists $img2
    run_podman rmi -f $img1 $img2
}

@test "podman load - multi-image archive with redirect" {
    img1="quay.io/libpod/testimage:00000000"
    img2="quay.io/libpod/testimage:20200902"
    archive=$PODMAN_TMPDIR/myimage-$(random_string 8).tar

    run_podman pull $img1
    run_podman pull $img2

    # We can't use run_podman because that uses the BATS 'run' function
    # which redirects stdout and stderr. Here we need to guarantee
    # that podman's stdout is a pipe, not any other form of redirection
    $PODMAN save -m $img1 $img2 | cat >$archive
    if [ "$status" -ne 0 ]; then
        die "Command failed: podman save ... | cat"
    fi

    run_podman rmi -f $img1 $img2
    run_podman load -i $archive

    run_podman image exists $img1
    run_podman image exists $img2
    run_podman rmi -f $img1 $img2
}

# vim: filetype=sh
