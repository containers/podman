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

# I'm sorry! This test takes 13 seconds. There's not much I can do about it,
# please know that I think it's justified: podman 1.5.0 had a strange bug
# in with exit status was not preserved on some code paths with 'rm -f'
# or 'podman run --rm' (see also 030-run.bats). The test below is a bit
# kludgy: what we care about is the exit status of the killed container,
# not 'podman rm', but BATS has no provision (that I know of) for forking,
# so what we do is start the 'rm' beforehand and monitor the exit status
# of the 'sleep' container.
#
# See https://github.com/containers/libpod/issues/3795
@test "podman rm -f" {
    rand=$(random_string 30)
    ( sleep 3; run_podman rm -f $rand ) &
    run_podman 137 run --name $rand $IMAGE sleep 30
}

# vim: filetype=sh
