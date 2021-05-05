#!/usr/bin/env bats   -*- bats -*-
#
# Basic tests for podman logs
#

load helpers

@test "podman logs - basic test" {
    rand_string=$(random_string 40)

    run_podman create $IMAGE echo $rand_string
    cid="$output"

    run_podman logs $cid
    is "$output" ""  "logs on created container: empty"

    run_podman start --attach --interactive $cid
    is "$output" "$rand_string" "output from podman-start on created ctr"
    is "$output" "$rand_string" "logs of started container"

    run_podman logs $cid
    is "$output" "$rand_string" "output from podman-logs after container is run"

    # test --since with Unix timestamps
    run_podman logs --since 1000 $cid

    run_podman rm $cid
}

function _log_test_multi() {
    local driver=$1

    skip_if_remote "logs does not support multiple containers when run remotely"

    # Under k8s file, 'podman logs' returns just the facts, Ma'am.
    # Under journald, there may be other cruft (e.g. container removals)
    local etc=
    if [[ $driver =~ journal ]]; then
        etc='.*'
    fi

    # Simple helper to make the container starts, below, easier to read
    local -a cid
    doit() {
        run_podman run --log-driver=$driver --rm -d --name "$1" $IMAGE sh -c "$2";
        cid+=($(echo "${output:0:12}"))
    }

    # Not really a guarantee that we'll get a-b-c-d in order, but it's
    # the best we can do. The trailing 'sleep' in each container
    # minimizes the chance of a race condition in which the container
    # is removed before 'podman logs' has a chance to wake up and read
    # the final output.
    doit c1         "echo a;sleep 10;echo d;sleep 3"
    doit c2 "sleep 1;echo b;sleep  2;echo c;sleep 3"

    run_podman logs -f c1 c2
    is "$output" \
       "${cid[0]} a$etc
${cid[1]} b$etc
${cid[1]} c$etc
${cid[0]} d"   "Sequential output from logs"
}

@test "podman logs - multi k8s-file" {
    _log_test_multi k8s-file
}

@test "podman logs - multi journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_multi journald
}

# vim: filetype=sh
