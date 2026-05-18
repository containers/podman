#!/bin/bash
#
# tests for pr-should-link-jira
#

ME=$(basename "$0")

###############################################################################
# BEGIN test cases
#

read -d '\n' msg_no_jira << EndOfText
This is some text
without a jira
EndOfText

read -d '\n' msg_invalid << EndOfText
This is some text
without a jira
Fixes #42
More text...
EndOfText

read -d '\n' msg_jira << EndOfText
This is some text
with a jira
Fixes https://issues.redhat.com/browse/RHEL-50507
More text...
EndOfText

read -d '\n' msg_jira2 << EndOfText
This is some text
with a jira
Fixes:  https://issues.redhat.com/browse/RHEL-50507
More text...
EndOfText

read -d '\n' msg_multiple << EndOfText
This is some text
with multiple jira lines
Fixes  https://issues.redhat.com/browse/RHEL-50507
More text...
Fixes: https://issues.redhat.com/browse/RHEL-50506
More text...
EndOfText

# Feel free to add as needed. Syntax is:
#    <exit status> <override> <commit message var> <dest branch> # comments
#
# Where:
#    exit status        is the expected exit status of the script
#    override           "true" if the "No Jira Link" label is set, else "false"
#    commit message     name of a variable above containing the PR body
#    dest branch        name of branch
#

tests="
0  false  msg_no_jira   main        not rhel branch, no link, should pass
0  false  msg_jira      main        not rhel branch, link, should pass
0  false  msg_invalid   main        not rhel branch, invalid link, should pass
0  false  msg_no_jira   v4.9        not rhel branch, no link, should pass
1  false  msg_no_jira   v4.9-rhel   no link, no override, should fail
0  true   msg_no_jira   v4.9-rhel   no link, override, should pass
1  false  msg_invalid   v4.9-rhel   invalid link, no override, should fail
0  false  msg_jira      v4.9-rhel   link, should pass
0  false  msg_jira2     v4.9-rhel   link with colon, should pass
0  false  msg_multiple  v4.9-rhel   multiple links, should pass
"

# The script we're testing
test_script=$(dirname "$0")/$(basename "$0" .t)

# END   test cases
###############################################################################
# BEGIN test-script runner and status checker

function run_test_script() {
    local expected_rc=$1
    local testname=$2

    testnum=$(( testnum + 1 ))

    # DO NOT COMBINE 'local output=...' INTO ONE LINE. If you do, you lose $?
    local output
    output=$( $test_script )
    local actual_rc=$?

    if [[ $actual_rc != $expected_rc ]]; then
        echo "not ok $testnum $testname"
        echo "#  expected rc $expected_rc"
        echo "#  actual rc   $actual_rc"
        if [[ -n "$output" ]]; then
            echo "# script output: $output"
        fi
        rc=1
    else
        if [[ $expected_rc == 1 ]]; then
            # Confirm we get an error message
            if [[ ! "$output" =~ "Please add a reference" ]]; then
                echo "not ok $testnum $testname"
                echo "# Expected: ~ 'Please add a reference'"
                echo "# Actual:   $output"
                rc=1
            else
                echo "ok $testnum $testname - rc=$expected_rc"
            fi
        else
            echo "ok $testnum $testname - rc=$expected_rc"
        fi
    fi
}

# END   test-script runner and status checker
###############################################################################
# BEGIN test-case parsing

rc=0
testnum=0

while read expected_rc override msg branch rest; do
    # Skip blank lines
    test -z "$expected_rc" && continue

    export DEST_BRANCH=$branch
    export PR_BODY="${!msg}"
    export OVERRIDE=$override

    run_test_script $expected_rc "$msg $branch override=$override - $rest"
done <<<"$tests"

echo "1..$testnum"
exit $rc

# END   Test-case parsing
###############################################################################
