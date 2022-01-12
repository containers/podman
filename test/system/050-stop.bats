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

# #9051 : podman stop --all was not working with podman-remote
@test "podman stop --all" {
    # Start three containers, create (without running) a fourth
    run_podman run -d --name c1 $IMAGE sleep 20
    run_podman run -d --name c2 $IMAGE sleep 40
    run_podman run -d --name c3 $IMAGE sleep 60
    run_podman create --name c4 $IMAGE sleep 80

    # podman ps (without -a) should show the three running containers
    run_podman ps --sort names --format '{{.Names}}--{{.Status}}'
    is "${#lines[*]}" "3"        "podman ps shows exactly three containers"
    is "${lines[0]}" "c1--Up.*"  "podman ps shows running container (1)"
    is "${lines[1]}" "c2--Up.*"  "podman ps shows running container (2)"
    is "${lines[2]}" "c3--Up.*"  "podman ps shows running container (3)"

    # Stop -a
    run_podman stop -a -t 1

    # Now podman ps (without -a) should show nothing.
    run_podman ps --format '{{.Names}}'
    is "$output" "" "podman ps, after stop -a, shows no running containers"

    # ...but with -a, containers are shown
    run_podman ps -a --sort names --format '{{.Names}}--{{.Status}}'
    is "${#lines[*]}" "4"        "podman ps -a shows exactly four containers"
    is "${lines[0]}" "c1--Exited.*"  "ps -a, first stopped container"
    is "${lines[1]}" "c2--Exited.*"  "ps -a, second stopped container"
    is "${lines[2]}" "c3--Exited.*"  "ps -a, third stopped container"
    is "${lines[3]}" "c4--Created.*" "ps -a, created container (unaffected)"
}

# #9051 : podman stop --ignore was not working with podman-remote
@test "podman stop --ignore" {
    name=thiscontainerdoesnotexist
    run_podman 125 stop $name
    is "$output" \
       "Error: no container with name or ID \"$name\" found: no such container" \
       "podman stop nonexistent container"

    run_podman stop --ignore $name
    is "$output" "" "podman stop nonexistent container, with --ignore"
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
    # ignores SIGTERM such that the Podman would wait 20 seconds for the stop
    # to finish.  This gives us enough time to try some commands and inspect
    # the container's status.

    run_podman run --name stopme -d $IMAGE sh -c \
        "trap 'echo Received SIGTERM, ignoring' SIGTERM; echo READY; while :; do sleep 0.2; done"

    wait_for_ready stopme

    local t0=$SECONDS
    # Stop the container, but do so in the background so we can inspect
    # the container status while it's stopping. Use $PODMAN because we
    # don't want the overhead and error checks of run_podman.
    $PODMAN stop -t 20 stopme &

    # Wait for container to acknowledge the signal. We can't use wait_for_output
    # because that aborts if .State.Running != true
    local timeout=5
    while [[ $timeout -gt 0 ]]; do
        run_podman logs stopme
        if [[ "$output" =~ "Received SIGTERM, ignoring" ]]; then
            break
        fi
        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            die "Timed out waiting for container to receive SIGERM"
        fi
        sleep 0.5
    done

    # Other commands can acquire the lock
    run_podman ps -a

    # The container state transitioned to "stopping"
    run_podman inspect --format '{{.State.Status}}' stopme
    is "$output" "stopping" "Status of container should be 'stopping'"

    # Time check: make sure we were able to run 'ps' before the container
    # exited. If this takes too long, it means ps had to wait for lock.
    local delta_t=$(( $SECONDS - t0 ))
    if [[ $delta_t -gt 5 ]]; then
        die "Operations took too long ($delta_t seconds)"
    fi

    run_podman kill stopme
    run_podman wait stopme

    # Exit code should be 137 as it was killed
    run_podman inspect --format '{{.State.ExitCode}}' stopme
    is "$output" "137" "Exit code of killed container"
}

@test "podman stop -t 1 Generate warning" {
    skip_if_remote "warning only happens on server side"
    run_podman run --rm --name stopme -d $IMAGE sleep 100
    run_podman stop -t 1 stopme
    is "$output" ".*StopSignal SIGTERM failed to stop container stopme in 1 seconds, resorting to SIGKILL"  "stopping container should print warning"
}

@test "podman stop --noout" {
    run_podman run --rm --name stopme -d $IMAGE top
    run_podman --noout stop -t 0 stopme
    is "$output" "" "output should be empty"
}
# vim: filetype=sh
