#!/usr/bin/env bats   -*- bats -*-
#
# test to make sure we use the correct podman pause process
#

load helpers

# Test for https://github.com/containers/podman/issues/17903
@test "podman uses different pause process with --tmpdir" {
    skip_if_not_rootless "pause process is only used as rootless"
    skip_if_remote "--tmpdir not supported via remote"

    # There are nasty bugs when we are not in the correct userns,
    # we have good reproducer to see how things can go wrong here:
    # https://github.com/containers/podman/issues/17903#issuecomment-1497232184
    # However in CI test I rather not kill the pause process, this likely just
    # causes more tests bugs, instead we will compare the actual namespace values

    run_podman unshare readlink /proc/self/ns/user
    default_ns="$output"

    run_podman --root $PODMAN_TMPDIR/root --runroot $PODMAN_TMPDIR/runroot --tmpdir $PODMAN_TMPDIR/tmp \
        unshare readlink /proc/self/ns/user
    assert "$output" != "$default_ns" "different --tmpdir must use different ns"

    # kill the pause process from our custom tmpdir so we do not leak it forever
    kill -9 $(cat $PODMAN_TMPDIR/tmp/pause.pid)
}
