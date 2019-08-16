#!/bin/bash

# This script attempts basic confirmation of functional networking
# by connecting to a set of essential external servers and failing
# if any cannot be reached.

source $(dirname $0)/lib.sh

while read host port
do
    if [[ "$port" -eq "443" ]]
    then
        item_test "SSL/TLS to $host:$port" "$(echo -n '' | openssl s_client -quiet -no_ign_eof -connect $host:$port &> /dev/null; echo $?)" -eq "0"
    else
        item_test "Connect to $host:$port" "$(nc -zv -w 13 $host $port &> /dev/null; echo $?)" -eq 0
    fi
done < ${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/required_host_ports.txt
