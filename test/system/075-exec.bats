#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman exec
#

load helpers

@test "podman exec - basic test" {
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


    # Specially defined situations: exec a dir, or no such command.
    # We don't check the full error message because runc & crun differ.
    run_podman 126 exec $cid /etc
    is "$output" ".*permission denied"  "podman exec /etc"
    run_podman 127 exec $cid /no/such/command
    is "$output" ".*such file or dir"   "podman exec /no/such/command"

    # Done
    run_podman exec $cid rm -f /$rand_filename

    run_podman wait $cid
    is "$output" "0"   "output from podman wait (container exit code)"

    run_podman rm $cid
}

@test "podman exec - leak check" {
    skip_if_remote "test is meaningless over remote"

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
    run_podman rm -t 0 -f $cid
}

# Issue #4785 - piping to exec statement - fixed in #4818
# Issue #5046 - piping to exec truncates results (actually a conmon issue)
@test "podman exec - cat from stdin" {
    run_podman run -d $IMAGE sh -c 'while [ ! -e /stop ]; do sleep 0.1;done'
    cid="$output"

    echo_string=$(random_string 20)
    run_podman exec -i $cid cat < <(echo $echo_string)
    is "$output" "$echo_string" "output read back from 'exec cat'"

    # #5046 - large file content gets lost via exec
    # Generate a large file with random content; get a hash of its content
    local bigfile=${PODMAN_TMPDIR}/bigfile
    dd if=/dev/urandom of=$bigfile bs=1024 count=1500
    expect=$(sha512sum $bigfile | awk '{print $1}')
    # Transfer it to container, via exec, make sure correct #bytes are sent
    run_podman exec -i $cid dd of=/tmp/bigfile bs=512 <$bigfile
    is "${lines[0]}" "3000+0 records in"  "dd: number of records in"
    is "${lines[1]}" "3000+0 records out" "dd: number of records out"
    # Verify sha. '% *' strips off the path, keeping only the SHA
    run_podman exec $cid sha512sum /tmp/bigfile
    is "${output% *}" "$expect " "SHA of file in container"

    # Clean up
    run_podman exec $cid touch /stop
    run_podman wait $cid
    run_podman rm $cid
}

# #6829 : add username to /etc/passwd inside container if --userns=keep-id
@test "podman exec - with keep-id" {
    # Multiple --userns options confirm command-line override (last one wins)
    run_podman run -d --userns=private --userns=keep-id $IMAGE sh -c \
               "echo READY;while [ ! -f /tmp/stop ]; do sleep 1; done"
    cid="$output"
    wait_for_ready $cid

    run_podman exec $cid id -un
    is "$output" "$(id -un)" "container is running as current user"

    run_podman exec --user=$(id -un) $cid touch /tmp/stop
    run_podman wait $cid
    run_podman rm $cid
}

# #11496: podman-remote loses output
@test "podman exec/run - missing output" {
    local bigfile=${PODMAN_TMPDIR}/bigfile
    local newfile=${PODMAN_TMPDIR}/newfile
    # create a big file, bigger than the 8K buffer size
    base64 /dev/urandom | head -c 20K > $bigfile

    run_podman run --rm -v $bigfile:/tmp/test:Z $IMAGE cat /tmp/test
    printf "%s" "$output" > $newfile
    # use cmp to compare the files, this is very helpful since it will
    # tell us the first wrong byte in case this fails
    run cmp $bigfile $newfile
    is "$output" "" "run output is identical with the file"

    run_podman run -d --stop-timeout 0 -v $bigfile:/tmp/test:Z $IMAGE sleep inf
    cid="$output"

    run_podman exec $cid cat /tmp/test
    printf "%s" "$output" > $newfile
    # use cmp to compare the files, this is very helpful since it will
    # tell us the first wrong byte in case this fails
    run cmp $bigfile $newfile
    is "$output" "" "exec output is identical with the file"

    # Clean up
    run_podman rm -t 0 -f $cid
}

# vim: filetype=sh
