#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman healthcheck
#
#

load helpers


# Helper function: run 'podman inspect' and check various given fields
function _check_health {
    local testname="$1"
    local tests="$2"
    local since="$3"
    local hc_status="$4"

    run_podman inspect --format "{{json .State.Healthcheck}}" healthcheck_c

    parse_table "$tests" | while read field expect;do
        actual=$(jq ".$field" <<<"$output")
        is "$actual" "$expect" "$testname - .State.Healthcheck.$field"
    done

    # Make sure we can read the healthcheck event in podman events (#20342)
    run_podman events --filter container=healthcheck_c --filter event=health_status \
        --since "$since" --stream=false --format "{{.HealthStatus}}"
    # Because the assert below would fail with "lines: bad array subscript" when
    # there are no events lets special case this to provide a more meaningful error.
    if [[ -z "$output" ]]; then
        die "no healthcheck events"
    fi
    assert "${lines[-1]}" == "$hc_status" "$testname - podman events health status"
}

@test "podman healthcheck" {
    _build_health_check_image healthcheck_i

    # Run that healthcheck image.
    run_podman run -d --name healthcheck_c \
               --health-cmd /healthcheck   \
               --health-interval 1s        \
               --health-retries 3          \
               --health-on-failure=kill    \
               healthcheck_i

    run_podman inspect healthcheck_c --format "{{.Config.HealthcheckOnFailureAction}}"
    is "$output" "kill" "on-failure action is set to kill"

    current_time=$(date --iso-8601=seconds)
    # We can't check for 'starting' because a 1-second interval is too
    # short; it could run healthcheck before we get to our first check.
    #
    # So, just force a healthcheck run, then confirm that it's running.
    run_podman healthcheck run healthcheck_c
    is "$output" "" "output from 'podman healthcheck run'"

    _check_health "All healthy" "
Status           | \"healthy\"
FailingStreak    | 0
Log[-1].ExitCode | 0
Log[-1].Output   | \"Life is Good on stdout\\\nLife is Good on stderr\"
" "$current_time" "healthy"

    current_time=$(date --iso-8601=seconds)
    # Force a failure
    run_podman exec healthcheck_c touch /uh-oh
    sleep 2

    _check_health "First failure" "
Status           | \"healthy\"
FailingStreak    | [123]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\"
" "$current_time" "healthy"

    current_time=$(date --iso-8601=seconds)
    # After three successive failures, container should no longer be healthy
    sleep 5
    _check_health "Three or more failures" "
Status           | \"unhealthy\"
FailingStreak    | [3456]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\"
" "$current_time" "unhealthy"

    # now the on-failure should kick in and kill the container
    run_podman wait healthcheck_c

    # Clean up
    run_podman rm -t 0 -f healthcheck_c
    run_podman rmi   healthcheck_i
}

@test "podman healthcheck - restart cleans up old state" {
    ctr="healthcheck_c"
    img="healthcheck_i"

    _build_health_check_image $img cleanfile
    run_podman run -d --name $ctr      \
           --health-cmd /healthcheck   \
           --health-retries=3          \
           --health-interval=disable   \
           $img

    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "0" "Failing streak of fresh container should be 0"

    # Get the healthcheck to fail
    run_podman exec $ctr touch /uh-oh
    run_podman 1 healthcheck run $ctr
    is "$output" "unhealthy" "output from 'podman healthcheck run'"
    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "1" "Failing streak after one failed healthcheck should be 1"

    run_podman container restart $ctr
    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "0" "Failing streak of restarted container should be 0 again"

    run_podman rm -f -t0 $ctr
    run_podman rmi $img
}

@test "podman wait --condition={healthy,unhealthy}" {
    ctr="healthcheck_c"
    img="healthcheck_i"
    wait_file="$PODMAN_TMPDIR/$(random_string).wait_for_me"
    _build_health_check_image $img

    for condition in healthy unhealthy;do
        rm -f $wait_file
        run_podman run -d --name $ctr      \
               --health-cmd /healthcheck   \
               --health-retries=1          \
               --health-interval=disable   \
               $img
        if [[ $condition == "unhealthy" ]];then
            # create the uh-oh file to let the health check fail
            run_podman exec $ctr touch /uh-oh
        fi

        # Wait for the container in the background and create the $wait_file to
        # signal the specified wait condition was met.
        (timeout --foreground -v --kill=5 5 $PODMAN wait --condition=$condition $ctr && touch $wait_file) &

        # Sleep 1 second to make sure above commands are running
        sleep 1
        if [[ -f $wait_file ]]; then
            die "the wait file should only be created after the container turned healthy"
        fi

        if [[ $condition == "healthy" ]];then
            run_podman healthcheck run $ctr
        else
            run_podman 1 healthcheck run $ctr
        fi
        wait_for_file $wait_file
        run_podman rm -f -t0 $ctr
    done

    run_podman rmi $img
}

@test "podman healthcheck --health-on-failure" {
    run_podman 125 create --health-on-failure=kill $IMAGE
    is "$output" "Error: cannot set on-failure action to kill without a health check"

    ctr="healthcheck_c"
    img="healthcheck_i"

    for policy in none kill restart stop;do
        if [[ $policy == "none" ]];then
           # Do not remove the /uh-oh file for `none` as we want to
           # demonstrate that no action was taken
           _build_health_check_image $img
        else
           _build_health_check_image $img cleanfile
        fi

        # Run that healthcheck image.
        run_podman run -d --name $ctr      \
               --health-cmd /healthcheck   \
               --health-retries=1          \
               --health-on-failure=$policy \
               --health-interval=disable   \
               $img

        # healthcheck should succeed
        run_podman healthcheck run $ctr

        # Now cause the healthcheck to fail
        run_podman exec $ctr touch /uh-oh

        # healthcheck should now fail, with exit status 1 and 'unhealthy' output
        run_podman 1 healthcheck run $ctr
        is "$output" "unhealthy" "output from 'podman healthcheck run' (policy: $policy)"

        if [[ $policy == "restart" ]];then
           # Make sure the container transitions back to running
           run_podman wait --condition=running $ctr
           run_podman inspect $ctr --format "{{.RestartCount}}"
           assert "${#lines[@]}" != 0 "Container has been restarted at least once"
           run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
           is "$output" "0" "Failing streak of restarted container should be 0 again"
           run_podman healthcheck run $ctr
        elif [[ $policy == "none" ]];then
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            # Container is still running and health check still broken
            is "$output" "running $policy" "container continued running"
            run_podman 1 healthcheck run $ctr
            is "$output" "unhealthy" "output from 'podman healthcheck run' (policy: $policy)"
        else
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            # kill and stop yield the container into a non-running state
            is "$output" ".* $policy" "container was stopped/killed (policy: $policy)"
            assert "$output" != "running $policy"
            # also make sure that it's not stuck in the stopping state
            assert "$output" != "stopping $policy"
        fi

        run_podman rm -f -t0 $ctr
        run_podman rmi -f $img
    done
}

@test "podman healthcheck --health-on-failure with interval" {
    ctr="healthcheck_c"

    for policy in stop kill restart ;do
        t0=$(date --iso-8601=seconds)
        run_podman run -d --name $ctr      \
               --health-cmd /bin/false     \
               --health-retries=1          \
               --health-on-failure=$policy \
               --health-interval=1s        \
               $IMAGE top

        if [[ $policy == "restart" ]];then
            # Sleeping for 2 seconds makes the test much faster than using
            # podman-wait which would compete with the container getting
            # restarted.
            sleep 2
            # Make sure the container transitions back to running
            run_podman wait --condition=running $ctr
            run_podman inspect $ctr --format "{{.RestartCount}}"
            assert "${#lines[@]}" != 0 "Container has been restarted at least once"
        else
            # kill and stop yield the container into a non-running state
            run_podman wait $ctr
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            is "$output" ".* $policy" "container was stopped/killed (policy: $policy)"
            assert "$output" != "running $policy"
            # also make sure that it's not stuck in the stopping state
            assert "$output" != "stopping $policy"
        fi

        run_podman rm -f -t0 $ctr
    done
}

# vim: filetype=sh
