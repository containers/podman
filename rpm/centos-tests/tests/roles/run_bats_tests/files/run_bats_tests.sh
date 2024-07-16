#!/bin/bash
#
# Run bats tests for a given $TEST_PACKAGE, e.g. buildah, podman
#
# This is invoked by the 'run_bats_tests' role; we assume that
# the package foo has a foo-tests subpackage which provides the
# directory /usr/share/foo/test/system, containing one or more .bats
# test files.
#

export PATH=/usr/local/bin:/usr/sbin:/usr/bin

FULL_LOG=/tmp/test.debug.log
BATS_LOG=/tmp/test.bats.log
rm -f $FULL_LOG $BATS_LOG
touch $FULL_LOG $BATS_LOG

exec &> $FULL_LOG

# Log program versions
echo "Packages:"
rpm -q ${TEST_PACKAGE} ${TEST_PACKAGE}-tests

echo "------------------------------"
printenv | sort

testdir=/usr/share/${TEST_PACKAGE}/test/system

if ! cd $testdir; then
    echo "FAIL ${TEST_NAME} : cd $testdir"      >> /tmp/test.log
    exit 0
fi

if [ -e /tmp/helper.sh ]; then
    echo "------------------------------"
    echo ". /tmp/helper.sh"
    . /tmp/helper.sh
fi

if [ "$(type -t setup)" = "function" ]; then
    echo "------------------------------"
    echo "\$ setup"
    setup
    if [ $? -ne 0 ]; then
        echo "FAIL ${TEST_NAME} : setup"       >> /tmp/test.log
        exit 0
    fi
fi

echo "------------------------------"
echo "\$ bats ."
bats . &> $BATS_LOG
rc=$?

echo "------------------------------"
echo "bats completed with status $rc"

status=PASS
if [ $rc -ne 0 ]; then
    status=FAIL
fi

echo "${status} ${TEST_NAME}" >> /tmp/test.log

if [ "$(type -t teardown)" = "function" ]; then
    echo "------------------------------"
    echo "\$ teardown"
    teardown
fi

# FIXME: for CI purposes, always exit 0. This allows subsequent tests.
exit 0
