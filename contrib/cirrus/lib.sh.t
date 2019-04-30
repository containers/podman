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
        echo "ok $testnum $3 = $MSG"
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

exit $rc
