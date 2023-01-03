#!/usr/bin/env bats

load helpers

# Command to run in each of the tests.
SLEEPLOOP='trap "echo BYE;exit 0" INT;echo READY;while :;do sleep 0.1;done'

# Main test code: wait for container to exist and be ready, send it a
# signal, wait for container to acknowledge and exit.
function _test_sigproxy() {
    local cname=$1
    local kidpid=$2

    # Wait for container to appear
    local timeout=5
    while :;do
          sleep 0.5
          run_podman '?' container exists $cname
          if [[ $status -eq 0 ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for container $cname to start"
          fi
    done

    # Now that container exists, wait for it to declare itself READY
    wait_for_ready $cname

    # Signal, and wait for container to exit
    kill -INT $kidpid
    local timeout=10
    while :;do
          sleep 0.5
          run_podman logs $cname
          if [[ "$output" =~ BYE ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for BYE from container"
          fi
    done

    run_podman rm -f -t0 $cname
}

# Each of the tests below does some setup, then invokes the above helper.

@test "podman sigproxy test: run" {
    # We're forced to use $PODMAN because run_podman cannot be backgrounded
    $PODMAN run -i --name c_run $IMAGE sh -c "$SLEEPLOOP" &
    local kidpid=$!

    _test_sigproxy c_run $kidpid
}

@test "podman sigproxy test: start" {
    run_podman create --name c_start $IMAGE sh -c "$SLEEPLOOP"

    # See above comments regarding $PODMAN and backgrounding
    $PODMAN start --attach c_start &
    local kidpid=$!

    _test_sigproxy c_start $kidpid
}

@test "podman sigproxy test: attach" {
    run_podman run -d --name c_attach $IMAGE sh -c "$SLEEPLOOP"

    # See above comments regarding $PODMAN and backgrounding
    $PODMAN attach c_attach &
    local kidpid=$!

    _test_sigproxy c_attach $kidpid
}

# vim: filetype=sh
