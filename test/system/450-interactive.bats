# -*- bats -*-
#
# tests of podman commands that require an interactive pty
#

load helpers

###############################################################################
# BEGIN setup/teardown

# Each test runs with its own PTY, managed by socat.
PODMAN_TEST_PTY=$(mktemp -u --tmpdir=${BATS_TMPDIR:-/tmp} podman_pty.XXXXXX)
PODMAN_DUMMY=$(mktemp -u --tmpdir=${BATS_TMPDIR:-/tmp} podman_dummy.XXXXXX)
PODMAN_SOCAT_PID=

function setup() {
    basic_setup

    # Create a pty. Run under 'timeout' because BATS reaps child processes
    # and if we exit before killing socat, bats will hang forever.
    timeout 10 socat \
            PTY,link=$PODMAN_TEST_PTY,raw,echo=0 \
            PTY,link=$PODMAN_DUMMY,raw,echo=0 &
    PODMAN_SOCAT_PID=$!

    # Wait for pty
    retries=5
    while [[ ! -e $PODMAN_TEST_PTY ]]; do
        retries=$(( retries - 1 ))
        if [[ $retries -eq 0 ]]; then
            die "Timed out waiting for $PODMAN_TEST_PTY"
        fi
        sleep 0.5
    done
}

function teardown() {
    if [[ -n $PODMAN_SOCAT_PID ]]; then
        kill $PODMAN_SOCAT_PID
        PODMAN_SOCAT_PID=
    fi
    rm -f $PODMAN_TEST_PTY $PODMAN_DUMMY_PTY

    basic_teardown
}

# END   setup/teardown
###############################################################################
# BEGIN tests

@test "podman detects correct tty size" {
    skip "FIXME: #10710. As of 2021-12-08, stty fails >75% of the time."

    # Set the pty to a random size. Make rows/columns odd/even, to guarantee
    # that they can never be the same
    rows=$(( 15 + RANDOM % 60 |   1 ))
    cols=$(( 15 + RANDOM % 60 & 126 ))
    stty rows $rows cols $cols <$PODMAN_TEST_PTY

    # ...and make sure stty under podman reads that.
    run_podman run -it --name mystty $IMAGE stty size <$PODMAN_TEST_PTY
    is "$output" "$rows $cols" "stty under podman run reads the correct dimensions"

    run_podman rm -f mystty

    # FIXME: the checks below are flaking a lot (see #10710).

    # check that the same works for podman exec
#    run_podman run -d --name mystty $IMAGE top
#    run_podman exec -it mystty stty size <$PODMAN_TEST_PTY
#    is "$output" "$rows $cols" "stty under podman exec reads the correct dimensions"
#
#    run_podman rm -f mystty
}


@test "podman load - will not read from tty" {
    run_podman 125 load <$PODMAN_TEST_PTY
    is "$output" \
       "Error: cannot read from terminal. Use command-line redirection" \
       "Diagnostic from 'podman load' without redirection or -i"
}


@test "podman run --tty -i failure with no tty" {
    run_podman run --tty -i --rm $IMAGE echo hello < /dev/null
    is "$output" ".*The input device is not a TTY.*" "-it _without_ a tty"

    run_podman run --tty -i --rm $IMAGE echo hello <$PODMAN_TEST_PTY
    is "$output" "hello" "-it _with_ a pty"

    run_podman run --tty=false -i --rm $IMAGE echo hello < /dev/null
    is "$output" "hello" "-tty=false: no warning"

    run_podman run --tty -i=false --rm $IMAGE echo hello < /dev/null
    is "$output" "hello" "-i=false: no warning"
}

# vim: filetype=sh
