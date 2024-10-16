# -*- bats -*-
#
# tests of podman commands that require an interactive pty
#

load helpers

# bats file_tags=ci:parallel

###############################################################################
# BEGIN setup/teardown

function setup() {
    basic_setup

    # Each test runs with its own PTY, managed by socat.
    PODMAN_TEST_PTY=$PODMAN_TMPDIR/podman_pty
    PODMAN_DUMMY=$PODMAN_TMPDIR/podman_dummy
    PODMAN_SOCAT_PID=

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
        assert $retries -gt 0 "Timed out waiting for $PODMAN_TEST_PTY"
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
    # Set the pty to a random size. Make rows/columns odd/even, to guarantee
    # that they can never be the same
    rows=$(( 15 + RANDOM % 60 |   1 ))
    cols=$(( 15 + RANDOM % 60 & 126 ))
    stty rows $rows cols $cols <$PODMAN_TEST_PTY

    CR=$'\r'

    # ...and make sure stty under podman reads that.
    # This flakes often ("stty: standard input"), so, retry.
    run_podman run -it --name mystty $IMAGE stty size <$PODMAN_TEST_PTY
    if [[ "$output" =~ stty ]]; then
        echo "# stty flaked, retrying: $output" >&3
        run_podman rm -f mystty
        sleep 1
        run_podman run -it --name mystty $IMAGE stty size <$PODMAN_TEST_PTY
    fi
    is "$output" "$rows $cols$CR" "stty under podman run reads the correct dimensions"

    run_podman rm -t 0 -f mystty

    # FIXME: the checks below are flaking a lot (see #10710).

    # check that the same works for podman exec
#    run_podman run -d --name mystty $IMAGE top
#    run_podman exec -it mystty stty size <$PODMAN_TEST_PTY
#    is "$output" "$rows $cols" "stty under podman exec reads the correct dimensions"
#
#    run_podman rm -t 0 -f mystty
}


@test "podman load - will not read from tty" {
    run_podman 125 load <$PODMAN_TEST_PTY
    is "$output" \
       "Error: cannot read from terminal, use command-line redirection or the --input flag" \
       "Diagnostic from 'podman load' without redirection or -i"
}


@test "podman run --tty -i failure with no tty" {
    run_podman 0+w run --tty -i --rm $IMAGE echo hello < /dev/null
    require_warning "The input device is not a TTY.*" "-it _without_ a tty"

    CR=$'\r'
    run_podman run --tty -i --rm $IMAGE echo hello <$PODMAN_TEST_PTY
    is "$output" "hello$CR" "-it _with_ a pty"

    run_podman run --tty=false -i --rm $IMAGE echo hello < /dev/null
    is "$output" "hello" "-tty=false: no warning"

    run_podman run --tty -i=false --rm $IMAGE echo hello < /dev/null
    is "$output" "hello$CR" "-i=false: no warning"
}


@test "podman run -l passthrough-tty" {
    skip_if_remote

    # Requires conmon 2.1.10 or greater
    want=2.1.10
    run_podman info --format '{{.Host.Conmon.Path}}'
    conmon_path="$output"
    conmon_version=$($conmon_path --version | sed -ne 's/^.* version //p')
    if ! printf "%s\n%s\n" "$want" "$conmon_version" | sort --check=quiet --version-sort; then
        skip "need conmon >= $want; have $conmon_version"
    fi

    run tty <$PODMAN_TEST_PTY
    expected_tty="$output"

    run_podman run --rm -v/dev:/dev --log-driver=passthrough-tty $IMAGE tty <$PODMAN_TEST_PTY
    is "$output" "$expected_tty" "passthrough-tty: tty matches"
}

# vim: filetype=sh
