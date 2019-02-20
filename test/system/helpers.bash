# -*- bash -*-

# Podman command to run; may be podman-remote
PODMAN=${PODMAN:-podman}

# Standard image to use for most tests
PODMAN_TEST_IMAGE_REGISTRY=${PODMAN_TEST_IMAGE_REGISTRY:-"quay.io"}
PODMAN_TEST_IMAGE_USER=${PODMAN_TEST_IMAGE_USER:-"libpod"}
PODMAN_TEST_IMAGE_NAME=${PODMAN_TEST_IMAGE_NAME:-"alpine_labels"}
PODMAN_TEST_IMAGE_TAG=${PODMAN_TEST_IMAGE_TAG:-"latest"}
PODMAN_TEST_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_TEST_IMAGE_TAG"

# Default timeout for a podman command.
PODMAN_TIMEOUT=${PODMAN_TIMEOUT:-60}

###############################################################################
# BEGIN setup/teardown tools

# Provide common setup and teardown functions, but do not name them such!
# That way individual tests can override with their own setup/teardown,
# while retaining the ability to include these if they so desire.

# Setup helper: establish a test environment with exactly the images needed
function basic_setup() {
    # Clean up all containers
    run_podman rm --all --force

    # Clean up all images except those desired
    found_needed_image=
    run_podman images --all --format '{{.Repository}}:{{.Tag}} {{.ID}}'
    for line in "${lines[@]}"; do
        set $line
        if [ "$1" == "$PODMAN_TEST_IMAGE_FQN" ]; then
            found_needed_image=1
        else
            echo "# setup_standard_environment: podman rmi $1 & $2" >&3
            podman rmi --force "$1" >/dev/null 2>&1 || true
            podman rmi --force "$2" >/dev/null 2>&1 || true
        fi
    done

    # Make sure desired images are present
    if [ -z "$found_needed_image" ]; then
        run_podman pull "$PODMAN_TEST_IMAGE_FQN"
    fi
}

# Basic teardown: remove all containers
function basic_teardown() {
    run_podman rm --all --force
}


# Provide the above as default methods.
function setup() {
    basic_setup
}

function teardown() {
    basic_teardown
}


# Helpers useful for tests running rmi
function archive_image() {
    local image=$1

    # FIXME: refactor?
    archive_basename=$(echo $1 | tr -c a-zA-Z0-9._- _)
    archive=$BATS_TMPDIR/$archive_basename.tar

    run_podman save -o $archive $image
}

function restore_image() {
    local image=$1

    archive_basename=$(echo $1 | tr -c a-zA-Z0-9._- _)
    archive=$BATS_TMPDIR/$archive_basename.tar

    run_podman restore $archive
}

# END   setup/teardown tools
###############################################################################
# BEGIN podman helpers

################
#  run_podman  #  Invoke $PODMAN, with timeout, using BATS 'run'
################
#
# This is the preferred mechanism for invoking podman: first, it
# invokes $PODMAN, which may be 'podman-remote' or '/some/path/podman'.
#
# Second, we use 'timeout' to abort (with a diagnostic) if something
# takes too long; this is preferable to a CI hang.
#
# Third, we log the command run and its output. This doesn't normally
# appear in BATS output, but it will if there's an error.
#
# Next, we check exit status. Since the normal desired code is 0,
# that's the default; but the first argument can override:
#
#     run_podman 125  nonexistent-subcommand
#     run_podman '?'  some-other-command       # let our caller check status
#
# Since we use the BATS 'run' mechanism, $output and $status will be
# defined for our caller.
#
function run_podman() {
    # Number as first argument = expected exit code; default 0
    expected_rc=0
    case "$1" in
        [0-9])           expected_rc=$1; shift;;
        [1-9][0-9])      expected_rc=$1; shift;;
        [12][0-9][0-9])  expected_rc=$1; shift;;
        '?')             expected_rc=  ; shift;;  # ignore exit code
    esac

    # stdout is only emitted upon error; this echo is to help a debugger
    echo "\$ $PODMAN $@"
    run timeout --foreground -v --kill=10 $PODMAN_TIMEOUT $PODMAN "$@"
    # without "quotes", multiple lines are glommed together into one
    echo "$output"
    if [ "$status" -ne 0 ]; then
        echo -n "[ rc=$status ";
        if [ -n "$expected_rc" ]; then
            if [ "$status" -eq "$expected_rc" ]; then
                echo -n "(expected) ";
            else
                echo -n "(** EXPECTED $expected_rc **) ";
            fi
        fi
        echo "]"
    fi

    if [ "$status" -eq 124 ]; then
        if expr "$output" : ".*timeout: sending" >/dev/null; then
            echo "*** TIMED OUT ***"
            false
        fi
    fi

    if [ -n "$expected_rc" ]; then
        if [ "$status" -ne "$expected_rc" ]; then
            die "FAIL: exit code is $status; expected $expected_rc"
        fi
    fi
}


# Wait for 'READY' in container output
function wait_for_ready {
    local cid=
    local sleep_delay=5
    local how_long=60

    # Arg processing. A single-digit number is how long to sleep between
    # iterations; a 2- or 3-digit number is the total time to wait; anything
    # else is the container ID or name to wait on.
    local i
    for i in "$@"; do
        if expr "$i" : '[0-9]\+$' >/dev/null; then
            if [ $i -le 9 ]; then
                sleep_delay=$i
            else
                how_long=$i
            fi
        else
            cid=$i
        fi
    done

    [ -n "$cid" ] || die "FATAL: wait_for_ready: no container name/ID in '$*'"

    t1=$(expr $SECONDS + $how_long)
    while [ $SECONDS -lt $t1 ]; do
        run_podman logs $cid
        if expr "$output" : ".*READY" >/dev/null; then
            return
        fi

        sleep $sleep_delay
    done

    die "FAIL: timed out waiting for READY from $cid"
}

# END   podman helpers
###############################################################################
# BEGIN miscellaneous tools

######################
#  skip_if_rootless  #  ...with an optional message
######################
function skip_if_rootless() {
    if [ "$(id -u)" -eq 0 ]; then
        return
    fi

    skip "${1:-not applicable under rootless podman}"
}


#########
#  die  #  Abort with helpful message
#########
function die() {
    echo "# $*" >&2
    false
}


########
#  is  #  Compare actual vs expected string; fail w/diagnostic if mismatch
########
#
# Compares given string against expectations, using 'expr' to allow patterns.
#
# Examples:
#
#   is "$actual" "$expected" "descriptive test name"
#   is "apple" "orange"  "name of a test that will fail in most universes"
#   is "apple" "[a-z]\+" "this time it should pass"
#
function is() {
    local actual="$1"
    local expect="$2"
    local testname="${3:-FIXME}"

    if [ -z "$expect" ]; then
        if [ -z "$actual" ]; then
            return
        fi
        die "$testname:\n#  expected no output; got %q\n" "$actual"
    fi

    if expr "$actual" : "$expect" >/dev/null; then
        return
    fi

    # This is a multi-line message, so let's format it ourself (not via die)
    printf "# $testname:\n#  expected: %q\n#    actual: %q\n"    \
           "$expect" "$actual" >&2
    false
}


############
#  dprint  #  conditional debug message
############
#
# Set PODMAN_TEST_DEBUG to the name of one or more functions you want to debug
#
# Examples:
#
#    $ PODMAN_TEST_DEBUG=parse_table bats .
#    $ PODMAN_TEST_DEBUG="test_podman_images test_podman_run" bats .
#
function dprint() {
    test -z "$PODMAN_TEST_DEBUG" && return

    caller="${FUNCNAME[1]}"

    # PODMAN_TEST_DEBUG is a space-separated list of desired functions
    # e.g. "parse_table test_podman_images" (or even just "table")
    for want in $PODMAN_TEST_DEBUG; do
        # Check if our calling function matches any of the desired strings
        if expr "$caller" : ".*$want" >/dev/null; then
            echo "# ${FUNCNAME[1]}() : $*" >&3
            return
        fi
    done
}


#################
#  parse_table  #  Split a table on '|' delimiters; return space-separated
#################
#
# See sample .bats scripts for examples. The idea is to list a set of
# tests in a table, then use simple logic to iterate over each test.
# Columns are separated using '|' (pipe character) because sometimes
# we need spaces in our fields.
#
function parse_table() {
    while read line; do
        test -z "$line" && continue

        declare -a row=()
        while read col; do
            dprint "col=<<$col>>"
            row+=("$col")
        done <  <(echo "$line" | tr '|' '\012' | sed -e 's/^ *//' -e 's/\\/\\\\/g')

        printf "%q " "${row[@]}"
        printf "\n"
    done <<<"$1"
}


###################
#  random_string  #  Returns a pseudorandom human-readable string
###################
#
# Numeric argument, if present, is desired length of string
#
function random_string() {
    local length=${1:-10}

    head /dev/urandom | tr -dc a-zA-Z0-9 | head -c$length
}

# END   miscellaneous tools
###############################################################################
