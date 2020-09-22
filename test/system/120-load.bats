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

@test "podman save to pipe and load" {
    # Generate a random name and tag (must be lower-case)
    local random_name=x$(random_string 12 | tr A-Z a-z)
    local random_tag=t$(random_string 7 | tr A-Z a-z)
    local fqin=localhost/$random_name:$random_tag
    run_podman tag $IMAGE $fqin

    archive=$PODMAN_TMPDIR/myimage-$(random_string 8).tar

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

    # FIXME: when/if 7337 gets fixed, load with a new tag
    if false; then
    local new_name=x$(random_string 14 | tr A-Z a-z)
    local new_tag=t$(random_string 6 | tr A-Z a-z)
    run_podman rmi $fqin
    fqin=localhost/$new_name:$new_tag
    run_podman load -i $archive $fqin
    run_podman images $fqin --format '{{.Repository}}:{{.Tag}}'
    is "$output" "$fqin" "image can be loaded with new name:tag"
    fi

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

@test "podman load - NAME and NAME:TAG arguments work" {
    get_iid_and_name
    run_podman save $iid -o $archive
    run_podman rmi $iid

    # Load with just a name (note: names must be lower-case)
    random_name=$(random_string 20 | tr A-Z a-z)
    run_podman load -i $archive $random_name
    verify_iid_and_name "localhost/$random_name:latest"

    # Load with NAME:TAG arg
    run_podman rmi $iid
    random_tag=$(random_string 10 | tr A-Z a-z)
    run_podman load -i $archive $random_name:$random_tag
    verify_iid_and_name "localhost/$random_name:$random_tag"

    # Cleanup: restore desired image name
    run_podman tag $iid $img_name
    run_podman rmi "$random_name:$random_tag"
}


@test "podman load - will not read from tty" {
    if [ ! -t 0 ]; then
        skip "STDIN is not a tty"
    fi

    run_podman 125 load
    is "$output" \
       "Error: cannot read from terminal. Use command-line redirection" \
       "Diagnostic from 'podman load' without redirection or -i"
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
