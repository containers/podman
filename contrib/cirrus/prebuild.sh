#!/bin/bash

set -eo pipefail

# This script attempts to confirm functional networking and
# connectivity to essential external servers.  It also verifies
# some basic environmental expectations and shell-script sanity.
# It's intended for use early on in the podman CI system, to help
# prevent wasting time on tests that can't succeed due to some
# outage, failure, or missed expectation.

set -a
source /etc/automation_environment
source $AUTOMATION_LIB_PATH/common_lib.sh
set +a

req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV AUTOMATION_LIB_PATH \
             SCRIPT_BASE CIRRUS_WORKING_DIR FEDORA_NAME UBUNTU_NAME \
             VM_IMAGE_NAME

# Defined by the CI system
# shellcheck disable=SC2154
cd $CIRRUS_WORKING_DIR

msg "Checking Cirrus YAML"
# Defined by CI config.
# shellcheck disable=SC2154
showrun $SCRIPT_BASE/cirrus_yaml_test.py

# Defined by CI config.
# shellcheck disable=SC2154
if [[ "${DISTRO_NV}" =~ fedora ]]; then
    msg "Checking shell scripts"
    showrun ooe.sh dnf install -y ShellCheck  # small/quick addition
    showrun shellcheck --format=tty \
        --shell=bash --external-sources \
        --enable add-default-case,avoid-nullary-conditions,check-unassigned-uppercase \
        --exclude SC2046,SC2034,SC2090,SC2064 \
        --wiki-link-count=0 --severity=warning \
        $SCRIPT_BASE/*.sh \
        ./.github/actions/check_cirrus_cron/* \
        hack/get_ci_vm.sh

    # Tests for lib.sh
    showrun ${SCRIPT_BASE}/lib.sh.t

    export PREBUILD=1
    showrun bash ${CIRRUS_WORKING_DIR}/.github/actions/check_cirrus_cron/test.sh
fi

msg "Checking 3rd party network service connectivity"
# shellcheck disable=SC2154
cat ${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/required_host_ports.txt | \
    while read host port
    do
        if [[ "$port" -eq "443" ]]
        then
            echo "SSL/TLS to $host:$port"
            echo -n '' | \
                err_retry 9 1000 "" openssl s_client -quiet -no_ign_eof -connect $host:$port
        else
            echo "Connect to $host:$port"
            err_retry 9 1000 1 nc -zv -w 13 $host $port
        fi
    done

# Verify we can pull metadata from a few key testing images on quay.io
# in the 'libpod' namespace.  This is mostly aimed at validating the
# quay.io service is up and responsive.  Images were hand-picked with
# egrep -ro 'quay.io/libpod/.+:latest' test | sort -u
TEST_IMGS=(\
    alpine:latest
    busybox:latest
    alpine_labels:latest
    alpine_nginx:latest
    alpine_healthcheck:latest
    badhealthcheck:latest
    cirros:latest
)

msg "Checking quay.io test image accessibility"
for testimg in "${TEST_IMGS[@]}"; do
    fqin="quay.io/libpod/$testimg"
    echo "    $fqin"
    # Belt-and-suspenders: Catch skopeo (somehow) returning False or null
    # in addition to "bad" (invalid) JSON.
    skopeo inspect --retry-times 5 "docker://$fqin" | jq -e . > /dev/null
done
