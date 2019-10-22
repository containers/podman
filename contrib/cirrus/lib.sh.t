#!/bin/bash
#
# Unit tests for some functions in lib.sh
#
source $(dirname $0)/lib.sh

# Iterator and return code; updated in test functions
testnum=0
rc=0

function check_result {
    testnum=$(expr $testnum + 1)
    MSG=$(echo "$1" | tr -d '*>\012'|sed -e 's/^ \+//')
    if [ "$MSG" = "$2" ]; then
        echo "ok $testnum $(echo $3) = $(echo $MSG)"
    else
        echo "not ok $testnum $3"
        echo "#  expected: $2"
        echo "#    actual: $MSG"
        rc=1
    fi
}

###############################################################################
# tests for die()

function test_die() {
    local input_status=$1
    local input_msg=$2
    local expected_status=$3
    local expected_msg=$4

    local msg
    msg=$(die $input_status "$input_msg")
    local status=$?

    check_result "$msg" "$expected_msg" "die $input_status $input_msg"
}

test_die 1 "a message" 1 "a message"
test_die 2 ""          2 "FATAL ERROR (but no message given!) in test_die()"
test_die '' ''         1 "FATAL ERROR (but no message given!) in test_die()"

###############################################################################
# tests for req_env_var()

function test_rev() {
    local input_args=$1
    local expected_status=$2
    local expected_msg=$3

    # bash gotcha: doing 'local msg=...' on one line loses exit status
    local msg
    msg=$(req_env_var $input_args)
    local status=$?

    check_result "$msg"    "$expected_msg"    "req_env_var $input_args"
    check_result "$status" "$expected_status" "req_env_var $input_args (rc)"
}

# error if called with no args
test_rev '' 1 'FATAL: req_env_var: invoked without arguments'

# error if desired envariable is unset
unset FOO BAR
test_rev FOO 9 'FATAL: test_rev() requires $FOO to be non-empty'
test_rev BAR 9 'FATAL: test_rev() requires $BAR to be non-empty'
# OK if desired envariable was unset
FOO=1
test_rev FOO 0 ''

# OK if multiple vars are non-empty
FOO="stuff"
BAR="things"
ENV_VARS="FOO BAR"
test_rev "$ENV_VARS" 0 ''
unset BAR

# ...but error if any single desired one is unset
test_rev "FOO BAR" 9 'FATAL: test_rev() requires $BAR to be non-empty'

# ...and OK if all args are set
BAR=1
test_rev "FOO BAR" 0 ''

###############################################################################
# tests for test_okay()

function test_item_test {
    local exp_msg=$1
    local exp_ret=$2
    local item=$3
    shift 3
    local test_args="$@"
    local msg
    msg=$(item_test "$item" "$@")
    local status=$?

    check_result "$msg"    "$exp_msg" "test_item $item $test_args"
    check_result "$status" "$exp_ret" "test_item $item $test_args (actual rc $status)"
}

# negative tests
test_item_test "FATAL: item_test() requires \$ITEM to be non-empty" 9 "" ""
test_item_test "FATAL: item_test() requires \$TEST_ARGS to be non-empty" 9 "foo" ""
test_item_test "not ok foo: -gt 5 ~= bar: too many arguments" 2 "foo" "-gt" "5" "~=" "bar"
test_item_test "not ok bar: a -ge 10: a: integer expression expected" 2 "bar" "a" "-ge" "10"
test_item_test "not ok basic logic: 0 -ne 0" 1 "basic logic" "0" "-ne" "0"

# positive tests
test_item_test "ok snafu" 0 "snafu" "foo" "!=" "bar"
test_item_test "ok foobar" 0 "foobar" "one two three" "=" "one two three"
test_item_test "ok oh boy" 0 "oh boy" "line 1
line2" "!=" "line 1

line2"
test_item_test "ok okay enough" 0 "okay enough" "line 1
line2" "=" "line 1
line2"

###############################################################################
# tests for is_release()

# N/B: Assuming tests run in their own process, so wiping out the local
# CIRRUS_BASE_SHA CIRRUS_CHANGE_IN_REPO and CIRRUS_TAG will be okay.
function test_is_release() {
    CIRRUS_BASE_SHA="$1"
    CIRRUS_CHANGE_IN_REPO="$2"
    CIRRUS_TAG="$3"
    local exp_status=$4
    local exp_msg=$5
    local msg
    msg=$(is_release)
    local status=$?

    check_result "$msg" "$exp_msg" "is_release(CIRRUS_BASE_SHA='$1' CIRRUS_CHANGE_IN_REPO='$2' CIRRUS_TAG='$3')"
    check_result "$status" "$exp_status" "is_release(...) returned $status"
}

#                FROM    TO    TAG    RET    MSG
#test_is_release ""      ""    ""     ""     ""

test_is_release  ""      ""    ""     "9"     "FATAL: is_release() requires \$CIRRUS_BASE_SHA to be non-empty"
test_is_release  "x"     ""    ""     "9"     "FATAL: is_release() requires \$CIRRUS_CHANGE_IN_REPO to be non-empty"

test_is_release  "unknown" "x" ""     "11"    "is_release() unusable range unknown..x or tag "
test_is_release  "x" "unknown" ""     "11"    "is_release() unusable range x..unknown or tag "
test_is_release  "x" "x" "unknown"    "11"    "is_release() unusable range x..x or tag unknown"

# Negative-testing git with this function is very difficult, assume it works
# test_is_release ... "is_release() failed to fetch tags"
# test_is_release ... "is_release() failed to parse tags"

BF_V1=$(git rev-parse v1.0.0^)
AT_V1=$(git rev-parse v1.0.0)
test_is_release  "$BF_V1" "$BF_V1" "v9.8.7-dev" "2"     "Found \$RELVER v9.8.7-dev"
test_is_release  "$BF_V1" "$AT_V1" "v9.8.7-dev" "2"     "Found \$RELVER v9.8.7-dev"
test_is_release  "$BF_V1" "$AT_V1" ""           "0"     "Found \$RELVER v1.0.0"

exit $rc
