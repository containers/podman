#!/usr/bin/env bats   -*- bats -*-

load helpers

@test "podman start --all - start all containers" {
    # Run a bunch of short-lived containers, with different --restart settings
    run_podman run -d $IMAGE /bin/true
    cid_none_implicit="$output"
    run_podman run -d --restart=no $IMAGE /bin/false
    cid_none_explicit="$output"
    run_podman run -d --restart=on-failure $IMAGE /bin/true
    cid_on_failure="$output"

    # Run one longer-lived one.
    run_podman run -d --restart=always $IMAGE sleep 20
    cid_always="$output"

    run_podman wait $cid_none_implicit $cid_none_explicit $cid_on_failure

    run_podman start --all
    is "$output" ".*$cid_none_implicit" "started: container with no --restart"
    is "$output" ".*$cid_none_explicit" "started: container with --restart=no"
    is "$output" ".*$cid_on_failure" "started: container with --restart=on-failure"
    assert "$output" !~ "$cid_always" \
           "podman start --all should not restart a running container"

    run_podman wait $cid_none_implicit $cid_none_explicit $cid_on_failure

    run_podman rm $cid_none_implicit $cid_none_explicit $cid_on_failure
    run_podman stop -t 1 $cid_always
    run_podman rm $cid_always
}

@test "podman start --all with incompatible options" {
    expected="Error: either start all containers or the container(s) provided in the arguments"
    run_podman 125 start --all 12333
    is "$output" "$expected" "start --all, with args, throws error"
}

@test "podman start --filter - start only containers that match the filter" {
    run_podman run -d $IMAGE /bin/true
    cid="$output"
    run_podman wait $cid

    run_podman start --filter restart-policy=always $cid
    is "$output" "" "CID of restart-policy=always container"

    run_podman start --filter restart-policy=none $cid
    is "$output" "$cid" "CID of restart-policy=none container"
}

@test "podman start --filter invalid-restart-policy - return error" {
    run_podman run -d $IMAGE /bin/true
    cid="$output"
    run_podman 125 start --filter restart-policy=fakepolicy $cid
    is "$output" "Error: fakepolicy invalid restart policy" \
       "CID of restart-policy=<not-exists> container"
}

@test "podman start --all --filter" {
    run_podman run -d $IMAGE /bin/true
    cid_exited_0="$output"
    run_podman run -d $IMAGE /bin/false
    cid_exited_1="$output"

    run_podman wait $cid_exited_0 $cid_exited_1
    run_podman start --all --filter exited=0
    is "$output" "$cid_exited_0"
}

@test "podman start print IDs or raw input" {
    # start --all must print the IDs
    run_podman create $IMAGE top
    ctrID="$output"
    run_podman start --all
    is "$output" "$ctrID"

    # start $input must print $input
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top
    run_podman start $cname
    is "$output" $cname

    run_podman rm -t 0 -f $ctrID $cname
}

# vim: filetype=sh
