#!/bin/bash
#
# Run bats tests for a given $TEST_PACKAGE, e.g. buildah, podman
#
# This is invoked by the 'run_bats_tests' role; we assume that
# the package foo has a foo-tests subpackage which provides the
# directory /usr/share/foo/test/system, containing one or more .bats
# test files.
#
# We create two files:
#
#    /tmp/test.summary.log - one-liner with FAIL, PASS, ERROR and a blurb
#    /tmp/test.bats.log    - full log of this script, plus the BATS run
#
export PATH=/usr/local/bin:/usr/sbin:/usr/bin

FULL_LOG=/tmp/test.bats.log
rm -f $FULL_LOG
touch $FULL_LOG

# Preserve output to a log file, but also emit on stdout. This covers
# RHEL (which preserves logfiles but runs ansible without --verbose)
# and Fedora (which hides logfiles but runs ansible --verbose).
exec &> >(tee -a $FULL_LOG)

# Log program versions
echo "Packages:"
echo "  Kernel: $(uname -r)"
rpm -qa |\
    grep -E 'toolbox|podman|conmon|containers-common|crun|runc|iptable|slirp|aardvark|netavark|containernetworking-plugins|systemd|container-selinux|passt' |\
    sort |\
    sed -e 's/^/  /'

divider='------------------------------------------------------------------'
echo $divider
printenv | sort
echo $divider
echo "ip addr:"
ip addr
echo $divider

testdir=/usr/share/${TEST_PACKAGE}/test/system

if ! cd $testdir; then
    echo "FAIL ${TEST_NAME} : cd $testdir"      > /tmp/test.summary.log
    exit 0
fi

if [[ $PODMAN =~ remote ]]; then
    ${PODMAN%%-remote} system service -t0 &>/dev/null &
    PODMAN_SERVER_PID=$!
fi

echo "\$ bats ."
bats .
rc=$?

if [[ -n "$PODMAN_SERVER_PID" ]]; then
    kill $PODMAN_SERVER_PID
fi

echo $divider
echo "bats completed with status $rc"

status=PASS
if [ $rc -ne 0 ]; then
    status=FAIL
fi

echo "${status} ${TEST_NAME}" > /tmp/test.summary.log

# FIXME: for CI purposes, always exit 0. This allows subsequent tests.
exit 0
