#!/usr/bin/env bats   -*- bats -*-
#
# test to make sure we use the correct podman pause process
#

load helpers

function _check_pause_process() {
    pause_pid=
    if [[ -z "$pause_pid_file" ]]; then
        return
    fi

    test -e $pause_pid_file || die "Pause pid file $pause_pid_file missing"

    # do not mark this variable as local; our parent expects it
    pause_pid=$(<$pause_pid_file)
    test -d /proc/$pause_pid || die "Pause process $pause_pid (from $pause_pid_file) is not running"

    assert "$(</proc/$pause_pid/comm)" =~ 'catatonit|podman pause' \
           "Pause process $pause_pid has an unexpected name"
}

# Test for https://github.com/containers/podman/issues/17903
@test "rootless podman only ever uses single pause process" {
    skip_if_not_rootless "pause process is only used as rootless"
    skip_if_remote "--tmpdir not supported via remote"

    # There are nasty bugs when we are not in the correct userns,
    # we have good reproducer to see how things can go wrong here:
    # https://github.com/containers/podman/issues/17903#issuecomment-1497232184

    # To prevent any issues we should only ever have a single pause process running,
    # regardless of any --root/-runroot/--tmpdir values.

    # System tests can execute in contexts without XDG; in those, we have to
    # skip the pause-pid-file checks.
    local pause_pid_file
    if [[ -n "$XDG_RUNTIME_DIR" ]]; then
        pause_pid_file="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
    fi

    # Baseline: get the current userns (one will be created on demand)
    local getns="unshare readlink /proc/self/ns/user"
    run_podman $getns
    local baseline_userns="$output"

    # A pause process will now be running
    _check_pause_process

    # Use podman system migrate to stop the currently running pause process
    run_podman system migrate

    # After migrate, there must be no pause process
    if [[ -n "$pause_pid_file" ]]; then
        test -e $pause_pid_file && die "Pause pid file $pause_pid_file still exists, even after podman system migrate"

        run kill -0 $pause_pid
        test $status -eq 0 && die "Pause process $pause_pid is still running even after podman system migrate"
    fi

    run_podman --root    $PODMAN_TMPDIR/root \
               --runroot $PODMAN_TMPDIR/runroot \
               --tmpdir  $PODMAN_TMPDIR/tmp \
               $getns
    tmpdir_userns="$output"

    # And now we should once again have a pause process
    _check_pause_process

    # and all podmans, with & without --tmpdir, should use the same ns
    run_podman $getns
    assert "$output" == "$tmpdir_userns" \
           "podman should use the same userns created using a tmpdir"

    run_podman --tmpdir $PODMAN_TMPDIR/tmp2 $getns
    assert "$output" == "$tmpdir_userns" \
           "podman with tmpdir2 should use the same userns created using a tmpdir"
}
