#!/usr/bin/env bats

load helpers

@test "podman pod top - containers in different PID namespaces" {
    skip_if_remote "podman-pod does not work with podman-remote"

    # With infra=false, we don't get a /pause container (we also
    # don't pull k8s.gcr.io/pause )
    no_infra='--infra=false'
    run_podman pod create $no_infra
    podid="$output"

    # Start two containers...
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid1="$output"
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid2="$output"

    # ...and wait for them to actually start.
    wait_for_output "PID \+PPID \+USER " $cid1
    wait_for_output "PID \+PPID \+USER " $cid2

    # Both containers have emitted at least one top-like line.
    # Now run 'pod top', and expect two 'top -d 2' processes running.
    run_podman pod top $podid
    is "$output" ".*root.*top -d 2.*root.*top -d 2" "two 'top' containers"

    # By default (podman pod create w/ default --infra) there should be
    # a /pause container.
    if [ -z "$no_infra" ]; then
        is "$output" ".*0 \+1 \+0 \+[0-9. ?s]\+/pause" "there is a /pause container"
    fi

    # Clean up
    run_podman pod rm -f $podid
}


# vim: filetype=sh
