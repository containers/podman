#!/usr/bin/env bats

load helpers

@test "podman sigkill" {
    $PODMAN run -i --name foo $IMAGE sh -c 'trap "echo BYE;exit 0" INT;echo READY;while :;do sleep 0.1;done' &
    local kidpid=$!

    # Wait for container to appear
    local timeout=5
    while :;do
          sleep 0.5
          run_podman '?' container exists foo
          if [[ $status -eq 0 ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for container to start"
          fi
    done

    wait_for_ready foo

    # Signal, and wait for container to exit
    kill -INT $kidpid
    local timeout=5
    while :;do
          sleep 0.5
          run_podman logs foo
          if [[ "$output" =~ BYE ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for BYE from container"
          fi
    done

    run_podman rm -f -t0 foo
}

# vim: filetype=sh
