#!/usr/bin/env bats

load helpers

# Very simple test
@test "podman stop - basic test" {
    run_podman run -d $IMAGE sleep 60
    cid="$output"

    # Run 'stop'. Time how long it takes.
    t0=$SECONDS
    run_podman stop $cid
    t1=$SECONDS

    # Confirm that container is stopped. Podman-remote unfortunately
    # cannot tell the difference between "stopped" and "exited", and
    # spits them out interchangeably, so we need to recognize either.
    run_podman inspect --format '{{.State.Status}} {{.State.ExitCode}}' $cid
    is "$output" "\\(stopped\|exited\\) \+137" \
       "Status and exit code of stopped container"

    # The initial SIGTERM is ignored, so this operation should take
    # exactly 10 seconds. Give it some leeway.
    delta_t=$(( $t1 - $t0 ))
    [ $delta_t -gt 8 ]  ||\
        die "podman stop: ran too quickly! ($delta_t seconds; expected >= 10)"
    [ $delta_t -le 14 ] ||\
        die "podman stop: took too long ($delta_t seconds; expected ~10)"

    run_podman rm $cid
}


# Test fallback


# Regression test for #2472
@test "podman stop - can trap signal" {
    # Because the --time and --timeout options can be wonky, try three
    # different variations of this test.
    for t_opt in '' '--time=5' '--timeout=5'; do
        # Run a simple container that logs output on SIGTERM
        run_podman run -d $IMAGE sh -c \
                   "trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo READY; while :; do sleep 1; done"
        cid="$output"
        wait_for_ready $cid

        # Run 'stop' against it...
        t0=$SECONDS
        run_podman stop $t_opt $cid
        t1=$SECONDS

        # ...the container should trap the signal, log it, and exit.
        run_podman logs $cid
        is "$output" ".*READY.*Received SIGTERM, finishing" "podman stop $t_opt"

        # Exit code should be 0, because container did its own exit
        run_podman inspect --format '{{.State.ExitCode}}' $cid
        is "$output" "0" "Exit code of stopped container"

        # The 'stop' command should return almost instantaneously
        delta_t=$(( $t1 - $t0 ))
        [ $delta_t -le 2 ] ||\
            die "podman stop: took too long ($delta_t seconds; expected <= 2)"

        run_podman rm $cid
    done
}

# Regression test for #8501
@test "podman stop - unlock while waiting for timeout" {
    # Test that the container state transitions to "stopping" and that other
    # commands can get the container's lock.  To do that, run a container that
    # ingores SIGTERM such that the Podman would wait 20 seconds for the stop
    # to finish.  This gives us enough time to try some commands and inspect
    # the container's status.

    run_podman run --name stopme -d $IMAGE sh -c \
        "trap 'echo Received SIGTERM, ignoring' SIGTERM; echo READY; while :; do sleep 1; done"

    # Stop the container in the background
    $PODMAN stop -t 20 stopme &

    # Other commands can acquire the lock
    run_podman ps -a

    # The container state transitioned to "stopping"
    run_podman inspect --format '{{.State.Status}}' stopme
    is "$output" "stopping" "Status of container should be 'stopping'"

    run_podman kill stopme

    # Exit code should be 137 as it was killed
    run_podman inspect --format '{{.State.ExitCode}}' stopme
    is "$output" "137" "Exit code of killed container"
}

# vim: filetype=sh
