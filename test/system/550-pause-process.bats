#!/usr/bin/env bats   -*- bats -*-
#
# test to make sure we use the correct podman pause process
#

load helpers
load helpers.registry
load helpers.sig-proxy

function setup_file() {
    # We have to stop the background registry here. These tests kill the podman pause
    # process which means commands after that are in a new one and when the cleanup
    # later tries to stop the registry container it will be in the wrong ns and can fail.
    # https://github.com/containers/podman/pull/21563#issuecomment-1960047648
    stop_registry
}

function _check_pause_process() {
    # do not mark this variable as local; our caller expects it
    pause_pid_file="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
    test -e $pause_pid_file || die "Pause pid file $pause_pid_file missing"

    # do not mark this variable as local; our caller expects it
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

    # Baseline: get the current userns (one will be created on demand)
    local getns="unshare readlink /proc/self/ns/user"
    run_podman $getns
    local baseline_userns="$output"

    # A pause process will now be running
    _check_pause_process

    # Use podman system migrate to stop the currently running pause process
    run_podman system migrate

    # After migrate, there must be no pause process
    test -e $pause_pid_file && die "Pause pid file $pause_pid_file still exists, even after podman system migrate"

    run kill -0 $pause_pid
    test $status -eq 0 && die "Pause process $pause_pid is still running even after podman system migrate"

    run_podman $(podman_isolation_opts ${PODMAN_TMPDIR}) $getns
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

# https://github.com/containers/podman/issues/16091
@test "rootless reexec with sig-proxy" {
    skip_if_not_rootless "pause process is only used as rootless"
    skip_if_remote "system migrate not supported via remote"

    # Use podman system migrate to stop the currently running pause process
    run_podman system migrate

    # We're forced to use $PODMAN because run_podman cannot be backgrounded
    # Also special logic to set a different argv0 to make sure the reexec still works:
    # https://github.com/containers/podman/issues/22672
    bash -c "exec -a argv0-podman $PODMAN run -i --name c_run $IMAGE sh -c '$SLEEPLOOP'" &
    local kidpid=$!

    _test_sigproxy c_run $kidpid

    # our container exits 0 so podman should too
    wait $kidpid || die "podman run exited $? instead of zero"
}


@test "rootless reexec with sig-proxy when rejoining userns from container" {
    skip_if_not_rootless "pause process is only used as rootless"
    skip_if_remote "unshare not supported via remote"

    # First let's run a container in the background to keep the userns active
    local cname1=c1_$(random_string)
    run_podman run -d --name $cname1 --uidmap 0:100:100 $IMAGE top

    run_podman unshare readlink /proc/self/ns/user
    userns="$output"

    # check for pause pid and then kill it
    _check_pause_process
    kill -9 $pause_pid

    # Now again directly start podman run and make sure it can forward signals
    # We're forced to use $PODMAN because run_podman cannot be backgrounded
    local cname2=c2_$(random_string)
    $PODMAN run -i --name $cname2 $IMAGE sh -c "$SLEEPLOOP" &
    local kidpid=$!

    _test_sigproxy $cname2 $kidpid

    # check pause process again
    _check_pause_process

    # our container exits 0 so podman should too
    wait $kidpid || die "podman run exited $? instead of zero"

    # Check that podman joined the same userns as it tries to use the one
    # from the running podman process in the background.
    run_podman unshare readlink /proc/self/ns/user
    assert "$output" == "$userns" "userns before/after kill is the same"

    run_podman rm -f -t0 $cname1
}

# regression test for https://issues.redhat.com/browse/RHEL-59620
@test "rootless userns can unmount netns properly" {
    skip_if_not_rootless "pause process is only used as rootless"
    skip_if_remote "system migrate not supported via remote"

    # Use podman system migrate to stop the currently running pause process
    run_podman system migrate

    # First run a container with a custom userns as this uses different netns setup logic.
    local cname=c-$(safename)
    run_podman run --userns keep-id --name $cname -d $IMAGE sleep 100

    # Now run a "normal" container without userns
    run_podman run --rm $IMAGE true

    # This used to hang trying to unmount the netns.
    run_podman rm -f -t0 $cname
}
