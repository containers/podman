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

# TODO: Pull images required during testing into /dev/null

# TODO: Refresh DNF package-cache into /dev/null
