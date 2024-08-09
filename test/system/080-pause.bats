#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman pause/unpause functionality
#

load helpers

# bats test_tags=distro-integration, ci:parallel
@test "podman pause/unpause" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman pause' (rootless) only works with cgroups v2"
    fi

    cname="c-$(safename)"
    run_podman run -d --name $cname $IMAGE \
               sh -c 'while :;do date +%s;sleep 1;done'
    cid="$output"
    # Wait for first time value
    wait_for_output '[0-9]\{10,\}' $cid

    # Pause container, sleep a bit, unpause, sleep again to give process
    # time to write a new post-restart time value. Pause by CID, unpause
    # by name, just to exercise code paths. While paused, check 'ps'
    # and 'inspect', then check again after restarting.
    run_podman --noout pause $cid
    is "$output" "" "output should be empty"
    run_podman inspect --format '{{.State.Status}}' $cid
    is "$output" "paused" "podman inspect .State.Status"
    sleep 3
    run_podman ps -a --format '{{.ID}} {{.Names}} {{.Status}}'
    assert "$output" =~ ".*${cid:0:12} $cname Paused.*" "podman ps on paused container"
    run_podman unpause $cname
    run_podman ps -a --format '{{.ID}} {{.Names}} {{.Status}}'
    assert "$output" =~ ".*${cid:0:12} $cname Up .*" "podman ps on resumed container"
    sleep 1

    # Get full logs, and iterate through them computing delta_t between entries
    run_podman logs $cid
    i=1
    max_delta=0
    while [ $i -lt ${#lines[*]} ]; do
        this_delta=$(( ${lines[$i]} - ${lines[$(($i - 1))]} ))
        if [ $this_delta -gt $max_delta ]; then
            max_delta=$this_delta
        fi
        i=$(( $i + 1 ))
    done

    # There should be a 3-4 second gap, *maybe* 5. Never 1 or 2, that
    # would imply that the container never paused.
    is "$max_delta" "[3456]" "delta t between paused and restarted"

    run_podman rm -t 0 -f $cname

    # Pause/unpause on nonexistent name or id - these should all fail
    run_podman 125 pause $cid
    assert "$output" =~ "no container with name or ID \"$cid\" found: no such container"
    run_podman 125 pause $cname
    assert "$output" =~ "no container with name or ID \"$cname\" found: no such container"
    run_podman 125 unpause $cid
    assert "$output" =~ "no container with name or ID \"$cid\" found: no such container"
    run_podman 125 unpause $cname
    assert "$output" =~ "no container with name or ID \"$cname\" found: no such container"
}

# CANNOT BE PARALLELIZED! (because of unpause --all)
# bats test_tags=distro-integration
@test "podman unpause --all" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman pause' (rootless) only works with cgroups v2"
    fi

    cname=$(random_string 10)
    run_podman create --name notrunning $IMAGE
    run_podman run -d --name $cname $IMAGE sleep 100
    cid="$output"
    run_podman pause $cid
    run_podman inspect --format '{{.State.Status}}' $cid
    is "$output" "paused" "podman inspect .State.Status"
    run_podman unpause --all
    is "$output" "$cid" "podman unpause output"
    run_podman ps --format '{{.ID}} {{.Names}} {{.Status}}'
    is "$output" "${cid:0:12} $cname Up.*" "podman ps on resumed container"
    run_podman stop -t 0 $cname
    run_podman rm -t 0 -f $cname
    run_podman rm -t 0 -f notrunning
}
# vim: filetype=sh
