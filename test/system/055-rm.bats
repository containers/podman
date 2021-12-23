#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman rm
#

load helpers

@test "podman rm" {
    rand=$(random_string 30)
    run_podman run --name $rand $IMAGE /bin/true

    # Don't care about output, just check exit status (it should exist)
    run_podman 0 inspect $rand

    # container should be in output of 'ps -a'
    run_podman ps -a
    is "$output" ".* $IMAGE .*/true .* $rand" "Container present in 'ps -a'"

    # Remove container; now 'inspect' should fail
    run_podman rm $rand
    run_podman 125 inspect $rand
}

@test "podman rm - running container, w/o and w/ force" {
    run_podman run -d $IMAGE sleep 5
    cid="$output"

    # rm should fail
    run_podman 2 rm $cid
    is "$output" "Error: cannot remove container $cid as it is running - running or paused containers cannot be removed without force: container state improper" "error message"

    # rm -f should succeed
    run_podman rm -t 0 -f $cid
}

@test "podman rm container from storage" {
    if is_remote; then
        skip "only applicable for local podman"
    fi
    rand=$(random_string 30)
    run_podman create --name $rand $IMAGE /bin/true

    # Create a container that podman does not know about
    external_cid=$(buildah from $IMAGE)

    # Plain 'exists' should fail, but should succeed with --external
    run_podman 1 container exists $external_cid
    run_podman container exists --external $external_cid

    # rm should succeed
    run_podman rm $rand $external_cid
}

@test "podman rm <-> run --rm race" {
    # A container's lock is released before attempting to stop it.  This opens
    # the window for race conditions that led to #9479.
    run_podman run --rm -d $IMAGE sleep infinity
    run_podman rm -af
}

@test "podman rm --depend" {
    run_podman create $IMAGE
    dependCid=$output
    run_podman create --net=container:$dependCid $IMAGE
    cid=$output
    run_podman 125 rm $dependCid
    is "$output" "Error: container $dependCid has dependent containers which must be removed before it:.*" "Fail to remove because of dependencies"
    run_podman rm --depend $dependCid
    is "$output" ".*$cid" "Container should have been removed"
    is "$output" ".*$dependCid" "Depend container should have been removed"
}

# I'm sorry! This test takes 13 seconds. There's not much I can do about it,
# please know that I think it's justified: podman 1.5.0 had a strange bug
# in with exit status was not preserved on some code paths with 'rm -f'
# or 'podman run --rm' (see also 030-run.bats). The test below is a bit
# kludgy: what we care about is the exit status of the killed container,
# not 'podman rm', but BATS has no provision (that I know of) for forking,
# so what we do is start the 'rm' beforehand and monitor the exit status
# of the 'sleep' container.
#
# See https://github.com/containers/podman/issues/3795
@test "podman rm -f" {
    rand=$(random_string 30)
    ( sleep 3; run_podman rm -t 0 -f $rand ) &
    run_podman 137 run --name $rand $IMAGE sleep 30
}

# vim: filetype=sh
