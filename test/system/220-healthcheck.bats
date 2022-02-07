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
    # Create an image with a healthcheck script; said script will
    # pass until the file /uh-oh gets created (by us, via exec)
    cat >${PODMAN_TMPDIR}/healthcheck <<EOF
#!/bin/sh

if test -e /uh-oh; then
    echo "Uh-oh on stdout!"
    echo "Uh-oh on stderr!" >&2
    exit 1
else
    echo "Life is Good on stdout"
    echo "Life is Good on stderr" >&2
    exit 0
fi
EOF

    cat >${PODMAN_TMPDIR}/entrypoint <<EOF
#!/bin/sh

while :; do
    sleep 1
done
EOF

    cat >${PODMAN_TMPDIR}/Containerfile <<EOF
FROM $IMAGE

COPY healthcheck /healthcheck
COPY entrypoint  /entrypoint

RUN  chmod 755 /healthcheck /entrypoint

CMD ["/entrypoint"]
EOF

    run_podman build -t healthcheck_i ${PODMAN_TMPDIR}

    # Run that healthcheck image.
    run_podman run -d --name healthcheck_c \
               --health-cmd /healthcheck   \
               --health-interval 1s        \
               --health-retries 3          \
               healthcheck_i

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

# vim: filetype=sh
