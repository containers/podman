#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
    GOSRC $GOSRC
    OS_RELEASE_ID $OS_RELEASE_ID
    OS_RELEASE_VER $OS_RELEASE_VER
    TEST_SET $TEST_SET
"

record_timestamp "integration test start"

clean_env

cd $GOSRC
case "$TEST_SET" in
    1_3) ;&  #  Continue to the next item
    2_3) include_ginkgo_tests ./test/e2e ./$SCRIPT_BASE/e2e/common ./$SCRIPT_BASE/e2e/$TEST_SET ;;
    3_3) exclude_ginkgo_tests ./test/e2e ./$SCRIPT_BASE/e2e/1_3 ./$SCRIPT_BASE/e2e/2_3 ;;
    *)
        echo "Unable to handle \$TEST_SET \"$TEST_SET\""
        exit 83
        ;;
esac

set -x
case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18)
        make install PREFIX=/usr ETCDIR=/etc
        make test-binaries
        SKIP_USERNS=1 make localintegration
        ;;
    fedora-29) ;&  # Continue to the next item
    fedora-28) ;&
    centos-7) ;&
    rhel-7)
        make install PREFIX=/usr ETCDIR=/etc
        make podman-remote
        install bin/podman-remote /usr/bin
        make test-binaries
        make localintegration
        ;;
    *) bad_os_id_ver ;;
esac

record_timestamp "integration test end"
