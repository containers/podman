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

    # test --until with Unix timestamps
    run_podman logs --until 1000 $cid

    run_podman rm $cid
}

function _log_test_tail() {
    local driver=$1

    run_podman run -d --log-driver=$driver $IMAGE sh -c "echo test1; echo test2"
    cid="$output"

    run_podman wait $cid
    run_podman logs --tail 1 --timestamps $cid
    log1="$output"
    assert "$log1" =~ "^[0-9-]+T[0-9:.]+([\+-][0-9:]+|Z) test2" \
           "logs should only show last line"

    # Sigh. I hate doing this, but podman-remote --timestamp only has 1-second
    # resolution (regular podman has sub-second). For the timestamps-differ
    # check below, we need to force a different second.
    if is_remote; then
        sleep 2
    fi

    run_podman restart $cid
    run_podman wait $cid

    run_podman logs -t --tail 1 $cid
    log2="$output"
    assert "$log2" =~ "^[0-9-]+T[0-9:.]+([\+-][0-9:]+|Z) test2" \
           "logs, after restart, shows only last line"

    assert "$log2" != "$log1" "log timestamps should differ"

    run_podman rm $cid
}

@test "podman logs - tail test, k8s-file" {
    _log_test_tail k8s-file
}

@test "podman logs - tail test, journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_tail journald
}

function _additional_events_backend() {
    local driver=$1
    # Since PR#10431, 'logs -f' with journald driver is only supported with journald events backend.
    if [[ $driver = "journald" ]]; then
        run_podman info --format '{{.Host.EventLogger}}' >/dev/null
        if [[ $output != "journald" ]]; then
            echo "--events-backend journald"
        fi
    fi
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

    local events_backend=$(_additional_events_backend $driver)

    # Simple helper to make the container starts, below, easier to read
    local -a cid
    doit() {
        run_podman ${events_backend} run --log-driver=$driver --rm -d --name "$1" $IMAGE sh -c "$2";
        cid+=($(echo "${output:0:12}"))
    }

    # Not really a guarantee that we'll get a-b-c-d in order, but it's
    # the best we can do. The trailing 'sleep' in each container
    # minimizes the chance of a race condition in which the container
    # is removed before 'podman logs' has a chance to wake up and read
    # the final output.
    doit c1         "echo a;sleep 10;echo d;sleep 3"
    doit c2 "sleep 1;echo b;sleep  2;echo c;sleep 3"

    run_podman ${events_backend} logs -f c1 c2
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

function _log_test_restarted() {
    local driver=$1
    local events_backend=$(_additional_events_backend $driver)
    run_podman run --log-driver=$driver ${events_backend} --name logtest $IMAGE sh -c 'start=0; if test -s log; then start=`tail -n 1 log`; fi; seq `expr $start + 1` `expr $start + 10` | tee -a log'
    # FIXME: #9597
    # run/start is flaking for remote so let's wait for the container condition
    # to stop wasting energy until the root cause gets fixed.
    run_podman container wait --condition=exited --condition=stopped logtest
    run_podman ${events_backend} start -a logtest
    logfile=$(mktemp -p ${PODMAN_TMPDIR} logfileXXXXXXXX)
    $PODMAN $_PODMAN_TEST_OPTS ${events_backend} logs -f logtest > $logfile
    expected=$(mktemp -p ${PODMAN_TMPDIR} expectedXXXXXXXX)
    seq 1 20  > $expected
    diff -u ${expected} ${logfile}
}

@test "podman logs restarted - k8s-file" {
    _log_test_restarted k8s-file
}

@test "podman logs restarted journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_restarted journald
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
    run_podman rm -t 1 -f test
}

@test "podman logs - since k8s-file" {
    _log_test_since k8s-file
}

@test "podman logs - since journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_since journald
}

function _log_test_until() {
    local driver=$1

    s_before="before_$(random_string)_${driver}"
    s_after="after_$(random_string)_${driver}"

    before=$(date --iso-8601=seconds)
    sleep 1
    run_podman run --log-driver=$driver -d --name test $IMAGE sh -c \
        "echo $s_before; trap 'echo $s_after; exit' SIGTERM; while :; do sleep 1; done"

    # sleep a second to make sure the date is after the first echo
    sleep 1
    run_podman stop test
    run_podman wait test

    # Sigh. Stupid journald has a lag. Wait a few seconds for it to catch up.
    retries=20
    s_both="$s_before
$s_after"
    while [[ $retries -gt 0 ]]; do
        run_podman logs test
        if [[ "$output" = "$s_both" ]]; then
            break
        fi
        retries=$((retries - 1))
        sleep 0.1
    done
    assert $retries -gt 0 \
           "Timed out waiting for before&after in podman logs: $output"

    run_podman logs --until $before test
    is "$output" "" "podman logs --until before"

    after=$(date --date='+1 second' --iso-8601=seconds)

    run_podman logs --until $after test
    is "$output" "$s_both" "podman logs --until after"
    run_podman rm -t 0 -f test
}

@test "podman logs - until k8s-file" {
    _log_test_until k8s-file
}

@test "podman logs - until journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_until journald
}

function _log_test_follow() {
    local driver=$1
    cname=$(random_string)
    contentA=$(random_string)
    contentB=$(random_string)
    contentC=$(random_string)
    local events_backend=$(_additional_events_backend $driver)

    if [[ -n "${events_backend}" ]]; then
        skip_if_remote "remote does not support --events-backend"
    fi

    # Note: it seems we need at least three log lines to hit #11461.
    run_podman ${events_backend} run --log-driver=$driver --name $cname $IMAGE sh -c "echo $contentA; echo $contentB; echo $contentC"
    run_podman ${events_backend} logs -f $cname
    is "$output" "$contentA
$contentB
$contentC" "logs -f on exitted container works"

    run_podman ${events_backend} rm -t 0 -f $cname
}

@test "podman logs - --follow k8s-file" {
    _log_test_follow k8s-file
}

@test "podman logs - --follow journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_follow journald
}
# vim: filetype=sh
