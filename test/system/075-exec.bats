#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman exec
#

load helpers

@test "podman exec - basic test" {
    skip_if_remote

    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    # Start a container. Write random content to random file, then stay
    # alive as long as file exists. (This test will remove that file soon.)
    run_podman run -d $IMAGE sh -c \
               "echo $rand_content >/$rand_filename;echo READY;while [ -f /$rand_filename ]; do sleep 1; done"
    cid="$output"
    wait_for_ready $cid

    run_podman exec $cid sh -c "cat /$rand_filename"
    is "$output" "$rand_content" "Can exec and see file in running container"

    run_podman exec $cid rm -f /$rand_filename

    run_podman wait $cid
    is "$output" "0"   "output from podman wait (container exit code)"

    run_podman rm $cid
}

@test "podman exec - leak check" {
    skip_if_remote

    # Start a container in the background then run exec command
    # three times and make sure no any exec pid hash file leak
    run_podman run -td $IMAGE /bin/sh
    cid="$output"

    is "$(check_exec_pid)" "" "exec pid hash file indeed doesn't exist"

    for i in {1..3}; do
        run_podman exec $cid /bin/true
    done

    is "$(check_exec_pid)" "" "there isn't any exec pid hash file leak"

    run_podman stop --time 1 $cid
    run_podman rm -f $cid
}

# Issue #4785 - piping to exec statement - fixed in #4818
@test "podman exec - cat from stdin" {
    skip_if_remote

    run_podman run -d $IMAGE sh -c 'while [ ! -e /stop ]; do sleep 0.1;done'
    cid="$output"

    echo_string=$(random_string 20)
    run_podman exec -i $cid cat < <(echo $echo_string)
    is "$output" "$echo_string" "output read back from 'exec cat'"

    run_podman exec $cid touch /stop
    run_podman wait $cid
    run_podman rm $cid
}

# vim: filetype=sh
