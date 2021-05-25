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
    if [[ $output =~ $cid_always ]]; then
        die "podman start --all restarted a running container"
    fi

    run_podman wait $cid_none_implicit $cid_none_explicit $cid_on_failure

    run_podman rm $cid_none_implicit $cid_none_explicit $cid_on_failure
    run_podman stop -t 1 $cid_always
    run_podman rm $cid_always
}

@test "podman start --all with incompatible options" {
    expected="Error: either start all containers or the container(s) provided in the arguments"
    run_podman 125 start --all 12333
    is "$output" "$expected" "start --all, with args, throws error"
    if ! is_remote; then
        run_podman 125 start --all --latest
        is "$output" "$expected" "podman start --all --latest"
    fi
}

@test "podman start --filter - start only containers that match the filter" {
    run_podman run -d $IMAGE /bin/true
    cid="$output"
    run_podman start --filter restart-policy=always $cid "CID of restart-policy=always container"
    is "$output" ""

    run_podman start --filter restart-policy=none $cid "CID of restart-policy=none container"
    is "$output" "$cid"
}

@test "podman start --filter invalid-restart-policy - return error" {
    run_podman run -d $IMAGE /bin/true
    cid="$output"
    run_podman 125 start --filter restart-policy=fakepolicy $cid "CID of restart-policy=<not-exists> container"
    is "$output" "Error: fakepolicy invalid restart policy"
}

# vim: filetype=sh
