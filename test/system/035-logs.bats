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

@test "podman logs - journald log driver requires journald events backend" {
    skip_if_remote "remote does not support --events-backend"
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    run_podman --events-backend=file run --log-driver=journald -d --name test --replace $IMAGE ls /
    run_podman --events-backend=file logs test
    run_podman 125 --events-backend=file logs --follow test
    is "$output" "Error: using --follow with the journald --log-driver but without the journald --events-backend (file) is not supported" "journald logger requires journald eventer"
}

function _log_test_since() {
    local driver=$1

    s_before="before_$(random_string)_${driver}"
    s_after="after_$(random_string)_${driver}"

    before=$(date --iso-8601=seconds)
    run_podman run --log-driver=$driver -d --name test $IMAGE sh -c \
        "echo $s_before; trap 'echo $s_after; exit' SIGTERM; while :; do sleep 1; done"

    # sleep a second to make sure the date is after the first echo
    sleep 1
    after=$(date --iso-8601=seconds)
    run_podman stop test

    run_podman logs test
    is "$output" \
        "$s_before
$s_after"

    run_podman logs --since $before test
    is "$output" \
        "$s_before
$s_after"

    run_podman logs --since $after test
    is "$output" "$s_after"
    run_podman rm -f test
}

@test "podman logs - since k8s-file" {
    _log_test_since k8s-file
}

@test "podman logs - since journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_since journald
}

# vim: filetype=sh
