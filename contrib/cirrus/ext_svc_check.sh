#!/bin/bash

set -eo pipefail

# This script attempts basic confirmation of functional networking
# by connecting to a set of essential external servers and failing
# if any cannot be reached.  It's intended for use early on in the
# podman CI system, to help prevent wasting time on tests that can't
# succeed due to some outage or another.

# shellcheck source=./contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

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
    skopeo inspect --retry-times 5 "docker://$fqin" | jq . > /dev/null
done
