# -*- bash -*-

# Podman command to run; may be podman-remote
PODMAN=${PODMAN:-podman}

# Standard image to use for most tests
PODMAN_TEST_IMAGE_REGISTRY=${PODMAN_TEST_IMAGE_REGISTRY:-"quay.io"}
PODMAN_TEST_IMAGE_USER=${PODMAN_TEST_IMAGE_USER:-"libpod"}
PODMAN_TEST_IMAGE_NAME=${PODMAN_TEST_IMAGE_NAME:-"testimage"}
PODMAN_TEST_IMAGE_TAG=${PODMAN_TEST_IMAGE_TAG:-"20220615"}
PODMAN_TEST_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_TEST_IMAGE_TAG"
PODMAN_TEST_IMAGE_ID=

# Remote image that we *DO NOT* fetch or keep by default; used for testing pull
# This has changed in 2021, from 0 through 3, various iterations of getting
# multiarch to work. It should change only very rarely.
PODMAN_NONLOCAL_IMAGE_TAG=${PODMAN_NONLOCAL_IMAGE_TAG:-"00000004"}
PODMAN_NONLOCAL_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_NONLOCAL_IMAGE_TAG"

# Because who wants to spell that out each time?
IMAGE=$PODMAN_TEST_IMAGE_FQN

# Default timeout for a podman command.
PODMAN_TIMEOUT=${PODMAN_TIMEOUT:-120}

# Prompt to display when logging podman commands; distinguish root/rootless
_LOG_PROMPT='$'
if [ $(id -u) -eq 0 ]; then
    _LOG_PROMPT='#'
fi

###############################################################################
# BEGIN setup/teardown tools

# Provide common setup and teardown functions, but do not name them such!
# That way individual tests can override with their own setup/teardown,
# while retaining the ability to include these if they so desire.

# Some CI systems set this to runc, overriding the default crun.
if [[ -n $OCI_RUNTIME ]]; then
    if [[ -z $CONTAINERS_CONF ]]; then
        # FIXME: BATS provides no mechanism for end-of-run cleanup[1]; how
        # can we avoid leaving this file behind when we finish?
        #   [1] https://github.com/bats-core/bats-core/issues/39
        export CONTAINERS_CONF=$(mktemp --tmpdir=${BATS_TMPDIR:-/tmp} podman-bats-XXXXXXX.containers.conf)
        cat >$CONTAINERS_CONF <<EOF
[engine]
runtime="$OCI_RUNTIME"
EOF
    fi
fi

# Setup helper: establish a test environment with exactly the images needed
function basic_setup() {
    # Clean up all containers
    run_podman rm -t 0 --all --force --ignore

    # ...including external (buildah) ones
    run_podman ps --all --external --format '{{.ID}} {{.Names}}'
    for line in "${lines[@]}"; do
        set $line
        echo "# setup(): removing stray external container $1 ($2)" >&3
        run_podman rm -f $1
    done

    # Clean up all images except those desired
    found_needed_image=
    run_podman images --all --format '{{.Repository}}:{{.Tag}} {{.ID}}'
    for line in "${lines[@]}"; do
        set $line
        if [ "$1" == "$PODMAN_TEST_IMAGE_FQN" ]; then
            if [[ -z "$PODMAN_TEST_IMAGE_ID" ]]; then
                # This will probably only trigger the 2nd time through setup
                PODMAN_TEST_IMAGE_ID=$2
            fi
            found_needed_image=1
        else
            # Always remove image that doesn't match by name
            echo "# setup(): removing stray image $1" >&3
            run_podman rmi --force "$1" >/dev/null 2>&1 || true

            # Tagged image will have same IID as our test image; don't rmi it.
            if [[ $2 != "$PODMAN_TEST_IMAGE_ID" ]]; then
                echo "# setup(): removing stray image $2" >&3
                run_podman rmi --force "$2" >/dev/null 2>&1 || true
            fi
        fi
    done

    # Make sure desired images are present
    if [ -z "$found_needed_image" ]; then
        run_podman pull "$PODMAN_TEST_IMAGE_FQN"
    fi

    # Argh. Although BATS provides $BATS_TMPDIR, it's just /tmp!
    # That's bloody worthless. Let's make our own, in which subtests
    # can write whatever they like and trust that it'll be deleted
    # on cleanup.
    # TODO: do this outside of setup, so it carries across tests?
    PODMAN_TMPDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-/tmp} podman_bats.XXXXXX)

    # In the unlikely event that a test runs is() before a run_podman()
    MOST_RECENT_PODMAN_COMMAND=
}

# Basic teardown: remove all pods and containers
function basic_teardown() {
    echo "# [teardown]" >&2
    run_podman '?' pod rm -t 0 --all --force --ignore
    run_podman '?'     rm -t 0 --all --force --ignore
    run_podman '?' network prune --force

    command rm -rf $PODMAN_TMPDIR
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

    # Remember command args, for possible use in later diagnostic messages
    MOST_RECENT_PODMAN_COMMAND="podman $*"

    # stdout is only emitted upon error; this echo is to help a debugger
    echo "$_LOG_PROMPT $PODMAN $*"
    # BATS hangs if a subprocess remains and keeps FD 3 open; this happens
    # if podman crashes unexpectedly without cleaning up subprocesses.
    run timeout --foreground -v --kill=10 $PODMAN_TIMEOUT $PODMAN $_PODMAN_TEST_OPTS "$@" 3>/dev/null
    # without "quotes", multiple lines are glommed together into one
    if [ -n "$output" ]; then
        echo "$output"
    fi
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
            # It's possible for a subtest to _want_ a timeout
            if [[ "$expected_rc" != "124" ]]; then
                echo "*** TIMED OUT ***"
                false
            fi
        fi
    fi

    if [ -n "$expected_rc" ]; then
        if [ "$status" -ne "$expected_rc" ]; then
            die "exit code is $status; expected $expected_rc"
        fi
    fi
}


# Wait for certain output from a container, indicating that it's ready.
function wait_for_output {
    local sleep_delay=5
    local how_long=$PODMAN_TIMEOUT
    local expect=
    local cid=

    # Arg processing. A single-digit number is how long to sleep between
    # iterations; a 2- or 3-digit number is the total time to wait; all
    # else are, in order, the string to expect and the container name/ID.
    local i
    for i in "$@"; do
        if expr "$i" : '[0-9]\+$' >/dev/null; then
            if [ $i -le 9 ]; then
                sleep_delay=$i
            else
                how_long=$i
            fi
        elif [ -z "$expect" ]; then
            expect=$i
        else
            cid=$i
        fi
    done

    [ -n "$cid" ] || die "FATAL: wait_for_output: no container name/ID in '$*'"

    t1=$(expr $SECONDS + $how_long)
    while [ $SECONDS -lt $t1 ]; do
        run_podman logs $cid
        logs=$output
        if expr "$logs" : ".*$expect" >/dev/null; then
            return
        fi

        # Barf if container is not running
        run_podman inspect --format '{{.State.Running}}' $cid
        if [ $output != "true" ]; then
            run_podman inspect --format '{{.State.ExitCode}}' $cid
            exitcode=$output
            die "Container exited (status: $exitcode) before we saw '$expect': $logs"
        fi

        sleep $sleep_delay
    done

    die "timed out waiting for '$expect' from $cid"
}

# Shortcut for the lazy
function wait_for_ready {
    wait_for_output 'READY' "$@"
}

######################
#  random_free_port  #  Pick an available port within a specified range
######################
function random_free_port() {
    local range=${1:-5000-5999}

    local port
    for port in $(shuf -i ${range}); do
        if port_is_free $port; then
            echo $port
            return
        fi
    done

    die "Could not find open port in range $range"
}

function random_free_port_range() {
    local size=${1?Usage: random_free_port_range SIZE (as in, number of ports)}

    local maxtries=10
    while [[ $maxtries -gt 0 ]]; do
        local firstport=$(random_free_port)
        local lastport=
        for i in $(seq 1 $((size - 1))); do
            lastport=$((firstport + i))
            if ! port_is_free $lastport; then
                echo "# port $lastport is in use; trying another." >&3
                lastport=
                break
            fi
        done
        if [[ -n "$lastport" ]]; then
            echo "$firstport-$lastport"
            return
        fi

        maxtries=$((maxtries - 1))
    done

    die "Could not find free port range with size $size"
}

function port_is_free() {
     local port=${1?Usage: port_is_free PORT}
    ! { exec {unused_fd}<> /dev/tcp/127.0.0.1/$port; } &>/dev/null
}

###################
#  wait_for_port  #  Returns once port is available on host
###################
function wait_for_port() {
    local host=$1                      # Probably "localhost"
    local port=$2                      # Numeric port
    local _timeout=${3:-5}              # Optional; default to 5 seconds

    # Wait
    while [ $_timeout -gt 0 ]; do
        { exec {unused_fd}<> /dev/tcp/$host/$port; } &>/dev/null && return
        sleep 1
        _timeout=$(( $_timeout - 1 ))
    done

    die "Timed out waiting for $host:$port"
}

# END   podman helpers
###############################################################################
# BEGIN miscellaneous tools

# Shortcuts for common needs:
function is_ubuntu() {
    grep -qiw ubuntu /etc/os-release
}

function is_rootless() {
    [ "$(id -u)" -ne 0 ]
}

function is_remote() {
    [[ "$PODMAN" =~ -remote ]]
}

function is_cgroupsv1() {
    # WARNING: This will break if there's ever a cgroups v3
    ! is_cgroupsv2
}

# True if cgroups v2 are enabled
function is_cgroupsv2() {
    cgroup_type=$(stat -f -c %T /sys/fs/cgroup)
    test "$cgroup_type" = "cgroup2fs"
}

# True if podman is using netavark
function is_netavark() {
    run_podman info --format '{{.Host.NetworkBackend}}'
    if [[ "$output" =~ netavark ]]; then
        return 0
    fi
    return 1
}

# Returns the OCI runtime *basename* (typically crun or runc). Much as we'd
# love to cache this result, we probably shouldn't.
function podman_runtime() {
    # This function is intended to be used as '$(podman_runtime)', i.e.
    # our caller wants our output. run_podman() messes with output because
    # it emits the command invocation to stdout, hence the redirection.
    run_podman info --format '{{ .Host.OCIRuntime.Name }}' >/dev/null
    basename "${output:-[null]}"
}

# rhbz#1895105: rootless journald is unavailable except to users in
# certain magic groups; which our testuser account does not belong to
# (intentional: that is the RHEL default, so that's the setup we test).
function journald_unavailable() {
    if ! is_rootless; then
        # root must always have access to journal
        return 1
    fi

    run journalctl -n 1
    if [[ $status -eq 0 ]]; then
        return 1
    fi

    if [[ $output =~ permission ]]; then
        return 0
    fi

    # This should never happen; if it does, it's likely that a subsequent
    # test will fail. This output may help track that down.
    echo "WEIRD: 'journalctl -n 1' failed with a non-permission error:"
    echo "$output"
    return 1
}

# Returns the name of the local pause image.
function pause_image() {
    # This function is intended to be used as '$(pause_image)', i.e.
    # our caller wants our output. run_podman() messes with output because
    # it emits the command invocation to stdout, hence the redirection.
    run_podman version --format "{{.Server.Version}}-{{.Server.Built}}" >/dev/null
    echo "localhost/podman-pause:$output"
}

# Wait for the pod (1st arg) to transition into the state (2nd arg)
function _ensure_pod_state() {
    for i in {0..5}; do
        run_podman pod inspect $1 --format "{{.State}}"
        if [[ $output == "$2" ]]; then
            return
        fi
        sleep 0.5
    done

    die "Timed out waiting for pod $1 to enter state $2"
}

# Wait for the container's (1st arg) running state (2nd arg)
function _ensure_container_running() {
    for i in {0..20}; do
        run_podman container inspect $1 --format "{{.State.Running}}"
        if [[ $output == "$2" ]]; then
            return
        fi
        sleep 0.5
    done

    die "Timed out waiting for container $1 to enter state running=$2"
}

###########################
#  _add_label_if_missing  #  make sure skip messages include rootless/remote
###########################
function _add_label_if_missing() {
    local msg="$1"
    local want="$2"

    if [ -z "$msg" ]; then
        echo
    elif expr "$msg" : ".*$want" &>/dev/null; then
        echo "$msg"
    else
        echo "[$want] $msg"
    fi
}

######################
#  skip_if_rootless  #  ...with an optional message
######################
function skip_if_rootless() {
    if is_rootless; then
        local msg=$(_add_label_if_missing "$1" "rootless")
        skip "${msg:-not applicable under rootless podman}"
    fi
}

######################
#  skip_if_not_rootless  #  ...with an optional message
######################
function skip_if_not_rootless() {
    if ! is_rootless; then
        local msg=$(_add_label_if_missing "$1" "rootful")
        skip "${msg:-not applicable under rootlfull podman}"
    fi
}

####################
#  skip_if_remote  #  ...with an optional message
####################
function skip_if_remote() {
    if is_remote; then
        local msg=$(_add_label_if_missing "$1" "remote")
        skip "${msg:-test does not work with podman-remote}"
    fi
}

########################
#  skip_if_no_selinux  #
########################
function skip_if_no_selinux() {
    if [ ! -e /usr/sbin/selinuxenabled ]; then
        skip "selinux not available"
    elif ! /usr/sbin/selinuxenabled; then
        skip "selinux disabled"
    fi
}

#######################
#  skip_if_cgroupsv1  #  ...with an optional message
#######################
function skip_if_cgroupsv1() {
    if ! is_cgroupsv2; then
        skip "${1:-test requires cgroupsv2}"
    fi
}

######################
#  skip_if_rootless_cgroupsv1  #  ...with an optional message
######################
function skip_if_rootless_cgroupsv1() {
    if is_rootless; then
        if ! is_cgroupsv2; then
            local msg=$(_add_label_if_missing "$1" "rootless cgroupvs1")
            skip "${msg:-not supported as rootless under cgroupsv1}"
        fi
    fi
}

##################################
#  skip_if_journald_unavailable  #  rhbz#1895105: rootless journald permissions
##################################
function skip_if_journald_unavailable {
    if journald_unavailable; then
        skip "Cannot use rootless journald on this system"
    fi
}

function skip_if_root_ubuntu {
    if is_ubuntu; then
        if ! is_remote; then
            if ! is_rootless; then
                 skip "Cannot run this test on rootful ubuntu, usually due to user errors"
            fi
        fi
    fi
}

#########
#  die  #  Abort with helpful message
#########
function die() {
    # FIXME: handle multi-line output
    echo "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"  >&2
    echo "#| FAIL: $*"                                           >&2
    echo "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^" >&2
    false
}

############
#  assert  #  Compare actual vs expected string; fail if mismatch
############
#
# Compares string (default: $output) against the given string argument.
# By default we do an exact-match comparison against $output, but there
# are two different ways to invoke us, each with an optional description:
#
#      assert               "EXPECT" [DESCRIPTION]
#      assert "RESULT" "OP" "EXPECT" [DESCRIPTION]
#
# The first form (one or two arguments) does an exact-match comparison
# of "$output" against "EXPECT". The second (three or four args) compares
# the first parameter against EXPECT, using the given OPerator. If present,
# DESCRIPTION will be displayed on test failure.
#
# Examples:
#
#   assert "this is exactly what we expect"
#   assert "${lines[0]}" =~ "^abc"  "first line begins with abc"
#
function assert() {
    local actual_string="$output"
    local operator='=='
    local expect_string="$1"
    local testname="$2"

    case "${#*}" in
        0)   die "Internal error: 'assert' requires one or more arguments" ;;
        1|2) ;;
        3|4) actual_string="$1"
             operator="$2"
             expect_string="$3"
             testname="$4"
             ;;
        *)   die "Internal error: too many arguments to 'assert'" ;;
    esac

    # Comparisons.
    # Special case: there is no !~ operator, so fake it via '! x =~ y'
    local not=
    local actual_op="$operator"
    if [[ $operator == '!~' ]]; then
        not='!'
        actual_op='=~'
    fi
    if [[ $operator == '=' || $operator == '==' ]]; then
        # Special case: we can't use '=' or '==' inside [[ ... ]] because
        # the right-hand side is treated as a pattern... and '[xy]' will
        # not compare literally. There seems to be no way to turn that off.
        if [ "$actual_string" = "$expect_string" ]; then
            return
        fi
    elif [[ $operator == '!=' ]]; then
        # Same special case as above
        if [ "$actual_string" != "$expect_string" ]; then
            return
        fi
    else
        if eval "[[ $not \$actual_string $actual_op \$expect_string ]]"; then
            return
        elif [ $? -gt 1 ]; then
            die "Internal error: could not process 'actual' $operator 'expect'"
        fi
    fi

    # Test has failed. Get a descriptive test name.
    if [ -z "$testname" ]; then
        testname="${MOST_RECENT_PODMAN_COMMAND:-[no test name given]}"
    fi

    # Display optimization: the typical case for 'expect' is an
    # exact match ('='), but there are also '=~' or '!~' or '-ge'
    # and the like. Omit the '=' but show the others; and always
    # align subsequent output lines for ease of comparison.
    local op=''
    local ws=''
    if [ "$operator" != '==' ]; then
        op="$operator "
        ws=$(printf "%*s" ${#op} "")
    fi

    # This is a multi-line message, which may in turn contain multi-line
    # output, so let's format it ourself to make it more readable.
    local actual_split
    IFS=$'\n' read -rd '' -a actual_split <<<"$actual_string" || true
    printf "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv\n"    >&2
    printf "#|     FAIL: %s\n" "$testname"                        >&2
    printf "#| expected: %s'%s'\n" "$op" "$expect_string"         >&2
    printf "#|   actual: %s'%s'\n" "$ws" "${actual_split[0]}"     >&2
    local line
    for line in "${actual_split[@]:1}"; do
        printf "#|         > %s'%s'\n" "$ws" "$line"              >&2
    done
    printf "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n"   >&2
    false
}

########
#  is  #  **DEPRECATED**; see assert() above
########
function is() {
    local actual="$1"
    local expect="$2"
    local testname="${3:-${MOST_RECENT_PODMAN_COMMAND:-[no test name given]}}"

    local is_expr=
    if [ -z "$expect" ]; then
        if [ -z "$actual" ]; then
            # Both strings are empty.
            return
        fi
        expect='[no output]'
    elif [[ "$actual" = "$expect" ]]; then
        # Strings are identical.
        return
    else
        # Strings are not identical. Are there wild cards in our expect string?
        if expr "$expect" : ".*[^\\][\*\[]" >/dev/null; then
            # There is a '[' or '*' without a preceding backslash.
            is_expr=' (using expr)'
        elif [[ "${expect:0:1}" = '[' ]]; then
            # String starts with '[', e.g. checking seconds like '[345]'
            is_expr=' (using expr)'
        fi
        if [[ -n "$is_expr" ]]; then
            if expr "$actual" : "$expect" >/dev/null; then
                return
            fi
        fi
    fi

    # This is a multi-line message, which may in turn contain multi-line
    # output, so let's format it ourself to make it more readable.
    local -a actual_split
    readarray -t actual_split <<<"$actual"
    printf "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv\n" >&2
    printf "#|     FAIL: $testname\n"                          >&2
    printf "#| expected: '%s'%s\n" "$expect" "$is_expr"        >&2
    printf "#|   actual: '%s'\n" "${actual_split[0]}"          >&2
    local line
    for line in "${actual_split[@]:1}"; do
        printf "#|         > '%s'\n" "$line"                   >&2
    done
    printf "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n" >&2
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
        done <  <(echo "$line" | sed -E -e 's/(^|\s)\|(\s|$)/\n /g' | sed -e 's/^ *//' -e 's/\\/\\\\/g')
        # the above seds:
        #   1) Convert '|' to newline, but only if bracketed by spaces or
        #      at beginning/end of line (this allows 'foo|bar' in tests);
        #   2) then remove leading whitespace;
        #   3) then double-escape all backslashes

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


###########################
#  random_rfc1918_subnet  #
###########################
#
# Use the class B set, because much of our CI environment (Google, RH)
# already uses up much of the class A, and it's really hard to test
# if a block is in use.
#
# This returns THREE OCTETS! It is up to our caller to append .0/24, .255, &c.
#
function random_rfc1918_subnet() {
    local retries=1024

    while [ "$retries" -gt 0 ];do
        local cidr=172.$(( 16 + $RANDOM % 16 )).$(( $RANDOM & 255 ))

        in_use=$(ip route list | fgrep $cidr)
        if [ -z "$in_use" ]; then
            echo "$cidr"
            return
        fi

        retries=$(( retries - 1 ))
    done

    die "Could not find a random not-in-use rfc1918 subnet"
}


#########################
#  find_exec_pid_files  #  Returns nothing or exec_pid hash files
#########################
#
# Return exec_pid hash files if exists, otherwise, return nothing
#
function find_exec_pid_files() {
    run_podman info --format '{{.Store.RunRoot}}'
    local storage_path="$output"
    if [ -d $storage_path ]; then
        find $storage_path -type f -iname 'exec_pid_*'
    fi
}


#############################
#  remove_same_dev_warning  #  Filter out useless warning from output
#############################
#
# On some CI systems, 'podman run --privileged' emits a useless warning:
#
#    WARNING: The same type, major and minor should not be used for multiple devices.
#
# This obviously screws us up when we look at output results.
#
# This function removes the warning from $output and $lines. We don't
# do a full string match because there's another variant of that message:
#
#    WARNING: Creating device "/dev/null" with same type, major and minor as existing "/dev/foodevdir/null".
#
# (We should never again see that precise error ever again, but we could
# see variants of it).
#
function remove_same_dev_warning() {
    # No input arguments. We operate in-place on $output and $lines

    local i=0
    local -a new_lines=()
    while [[ $i -lt ${#lines[@]} ]]; do
        if expr "${lines[$i]}" : 'WARNING: .* same type, major' >/dev/null; then
            :
        else
            new_lines+=("${lines[$i]}")
        fi
        i=$(( i + 1 ))
    done

    lines=("${new_lines[@]}")
    output=$(printf '%s\n' "${lines[@]}")
}

# run 'podman help', parse the output looking for 'Available Commands';
# return that list.
function _podman_commands() {
    dprint "$@"
    # &>/dev/null prevents duplicate output
    run_podman help "$@" &>/dev/null
    awk '/^Available Commands:/{ok=1;next}/^Options:/{ok=0}ok { print $1 }' <<<"$output" | grep .
}

# END   miscellaneous tools
###############################################################################
