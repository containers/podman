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

    run_podman inspect --format "{{json .State.Healthcheck}}" healthcheck_c

    parse_table "$tests" | while read field expect;do
        actual=$(jq ".$field" <<<"$output")
        is "$actual" "$expect" "$testname - .State.Healthcheck.$field"
    done
}

@test "podman healthcheck" {
    _build_health_check_image healthcheck_i

    # Run that healthcheck image.
    run_podman run -d --name healthcheck_c \
               --health-cmd /healthcheck   \
               --health-interval 1s        \
               --health-retries 3          \
               healthcheck_i

    run_podman inspect healthcheck_c --format "{{.Config.HealthcheckOnFailureAction}}"
    is "$output" "none" "default on-failure action is none"

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
"

    # Force a failure
    run_podman exec healthcheck_c touch /uh-oh
    sleep 2

    _check_health "First failure" "
Status           | \"healthy\"
FailingStreak    | [123]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\"
"

    # After three successive failures, container should no longer be healthy
    sleep 5
    _check_health "Three or more failures" "
Status           | \"unhealthy\"
FailingStreak    | [3456]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\"
"

    # healthcheck should now fail, with exit status 1 and 'unhealthy' output
    run_podman 1 healthcheck run healthcheck_c
    is "$output" "unhealthy" "output from 'podman healthcheck run'"

    # Clean up
    run_podman rm -t 0 -f healthcheck_c
    run_podman rmi   healthcheck_i
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
               --health-on-failure=$policy \
               $img

        # healthcheck should succeed
        run_podman healthcheck run $ctr

        # Now cause the healthcheck to fail
        run_podman exec $ctr touch /uh-oh

        # healthcheck should now fail, with exit status 1 and 'unhealthy' output
        run_podman 1 healthcheck run $ctr
	# FIXME: #15691 - `healthcheck run` may emit an error log that the timer already exists
        is "$output" ".*unhealthy.*" "output from 'podman healthcheck run'"

        run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
	if [[ $policy == "restart" ]];then
	    # Container has been restarted and health check works again
            is "$output" "running $policy" "container has been restarted"
            run_podman healthcheck run $ctr
        elif [[ $policy == "none" ]];then
            # Container is still running and health check still broken
            is "$output" "running $policy" "container continued running"
            run_podman 1 healthcheck run $ctr
	    # FIXME: #15691 - `healthcheck run` may emit an error log that the timer already exists
            is "$output" ".*unhealthy.*" "output from 'podman healthcheck run'"
	else
	    # kill and stop yield the container into a non-running state
            is "$output" ".* $policy" "container was stopped/killed"
            if [[ "$output" == "running $policy" ]]; then
                die "container is still in running mode"
            fi
        fi

        run_podman rm -f -t0 $ctr
        run_podman rmi -f $img
    done
}

# vim: filetype=sh
