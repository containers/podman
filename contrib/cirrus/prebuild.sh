#!/bin/bash

set -eo pipefail

# This script attempts to confirm functional networking and
# connectivity to essential external servers.  It also verifies
# some basic environmental expectations and shell-script sanity.
# It's intended for use early on in the podman CI system, to help
# prevent wasting time on tests that can't succeed due to some
# outage, failure, or missed expectation.

source /etc/automation_environment
source $AUTOMATION_LIB_PATH/common_lib.sh

req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV AUTOMATION_LIB_PATH \
             SCRIPT_BASE CIRRUS_WORKING_DIR FEDORA_NAME UBUNTU_NAME \
             VM_IMAGE_NAME

# There's no need to perform further checks on more than one
# CI platform.  These variables are defined in .cirrus.yml
# shellcheck disable=SC2154
if [[ ! "${DISTRO_NV}" =~ ${FEDORA_NAME} ]]; then
    echo "Skipping additional checks on $DISTRO_NV"
    exit 0
fi

# shellcheck disable=SC2154
$SCRIPT_BASE/cirrus_yaml_test.py

ooe.sh dnf install -y ShellCheck  # small/quick addition

shellcheck --color=always --format=tty \
    --shell=bash --external-sources \
    --enable add-default-case,avoid-nullary-conditions,check-unassigned-uppercase \
    --exclude SC2046,SC2034,SC2090,SC2064 \
    --wiki-link-count=0 --severity=warning \
    $SCRIPT_BASE/*.sh hack/get_ci_vm.sh

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

echo "Checking quay.io test image accessibility"
for testimg in "${TEST_IMGS[@]}"; do
    fqin="quay.io/libpod/$testimg"
    echo "    $fqin"
    skopeo inspect --retry-times 5 "docker://$fqin" | jq -e . > /dev/null
done
