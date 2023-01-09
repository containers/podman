#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman rm
#

load helpers

@test "podman rm" {
    rand=$(random_string 30)
    run_podman run --name $rand $IMAGE /bin/true

    # Don't care about output, just check exit status (it should exist)
    run_podman 0 inspect $rand

    # container should be in output of 'ps -a'
    run_podman ps -a
    is "$output" ".* $IMAGE .*/true .* $rand" "Container present in 'ps -a'"

    # Remove container; now 'inspect' should fail
    run_podman rm $rand
    is "$output" "$rand" "display raw input"
    run_podman 125 inspect $rand
    run_podman 125 wait $rand
    run_podman wait --ignore $rand
    is "$output" "-1" "wait --ignore will mark missing containers with -1"

    if !is_remote; then
        # remote does not support the --latest flag
        run_podman wait --ignore --latest
        is "$output" "-1" "wait --ignore will mark missing containers with -1"
    fi
}

@test "podman rm - running container, w/o and w/ force" {
    run_podman run -d $IMAGE sleep 5
    cid="$output"

    # rm should fail
    run_podman 2 rm $cid
    is "$output" "Error: cannot remove container $cid as it is running - running or paused containers cannot be removed without force: container state improper" "error message"

    # rm -f should succeed
    run_podman rm -t 0 -f $cid
}

@test "podman rm container from storage" {
    if is_remote; then
        skip "only applicable for local podman"
    fi
    rand=$(random_string 30)
    run_podman create --name $rand $IMAGE /bin/true

    # Create a container that podman does not know about
    external_cid=$(buildah from $IMAGE)

    # Plain 'exists' should fail, but should succeed with --external
    run_podman 1 container exists $external_cid
    run_podman container exists --external $external_cid

    # rm should succeed
    run_podman rm $rand $external_cid
}

@test "podman rm <-> run --rm race" {
    OCIDir=/run/$(podman_runtime)

    if is_rootless; then
        OCIDir=/run/user/$(id -u)/$(podman_runtime)
    fi

    # A container's lock is released before attempting to stop it.  This opens
    # the window for race conditions that led to #9479.
    run_podman run --rm -d $IMAGE sleep infinity
    local cid="$output"
    run_podman rm -af

    # Check the OCI runtime directory has removed.
    is "$(ls $OCIDir | grep $cid)" "" "The OCI runtime directory should have been removed"
}

@test "podman rm --depend" {
    run_podman create $IMAGE
    dependCid=$output
    run_podman create --net=container:$dependCid $IMAGE
    cid=$output
    run_podman 125 rm $dependCid
    is "$output" "Error: container $dependCid has dependent containers which must be removed before it:.*" "Fail to remove because of dependencies"
    run_podman rm --depend $dependCid
    is "$output" ".*$cid" "Container should have been removed"
    is "$output" ".*$dependCid" "Depend container should have been removed"
}

# I'm sorry! This test takes 13 seconds. There's not much I can do about it,
# please know that I think it's justified: podman 1.5.0 had a strange bug
# in with exit status was not preserved on some code paths with 'rm -f'
# or 'podman run --rm' (see also 030-run.bats). The test below is a bit
# kludgy: what we care about is the exit status of the killed container,
# not 'podman rm', but BATS has no provision (that I know of) for forking,
# so what we do is start the 'rm' beforehand and monitor the exit status
# of the 'sleep' container.
#
# See https://github.com/containers/podman/issues/3795
@test "podman rm -f" {
    rand=$(random_string 30)
    ( sleep 3; run_podman rm -t 0 -f $rand ) &
    run_podman 137 run --name $rand $IMAGE sleep 30
}

@test "podman container rm --force bogus" {
    run_podman 1 container rm bogus
    is "$output" "Error: no container with ID or name \"bogus\" found: no such container" "Should print error"
    run_podman container rm --force bogus
    is "$output" "" "Should print no output"
}

function __run_healthcheck_container() {
    run_podman run -d --name $1 \
               --health-cmd /bin/false \
               --health-interval 1s \
               --health-retries 2 \
               --health-timeout 1s \
               --health-on-failure=stop \
               --stop-timeout=2 \
               --health-start-period 0 \
               --stop-signal SIGTERM \
               $IMAGE sleep infinity
}

@test "podman container rm doesn't affect stopping containers" {
    local cname=c$(random_string 30)
    __run_healthcheck_container $cname
    local cid=$output

    # We'll use the PID later to confirm that container is not running
    run_podman inspect --format '{{.State.Pid}}' $cname
    local pid=$output

    # rm without -f should fail, because container is running/stopping.
    # We have no way to guarantee that we see 'stopping', but at a very
    # minimum we need to check at least one rm failure
    local rm_failures=0
    for i in {1..10}; do
        run_podman '?' rm $cname
        if [[ $status -eq 0 ]]; then
            break
        fi

        # rm failed. Confirm that it's for the right reason.
        assert "$output" =~ "Error: cannot remove container $cid as it is .* - running or paused containers cannot be removed without force: container state improper" \
               "Expected error message from podman rm"
        rm_failures=$((rm_failures + 1))
        sleep 1
    done

    # At this point, container should be gone
    run_podman 1 container exists $cname
    run_podman 1 container exists $cid

    assert "$rm_failures" -gt 0 "we want at least one failure from podman-rm"

    if kill -0 $pid; then
        die "Container $cname process is still running (pid $pid)"
    fi
}

@test "podman container rm --force doesn't leave running processes" {
    local cname=c$(random_string 30)
    __run_healthcheck_container $cname
    local cid=$output

    # We'll use the PID later to confirm that container is not running
    run_podman inspect --format '{{.State.Pid}}' $cname
    local pid=$output

    for i in {1..10}; do
        run_podman inspect $cname --format '{{.State.Status}}'
        if [ "$output" = "stopping" ]; then
            break
        fi

	sleep 0.5
    done

    run_podman rm -f $cname

    if kill -0 $pid; then
        die "Container $cname process is still running (pid $pid)"
    fi
}

# vim: filetype=sh
