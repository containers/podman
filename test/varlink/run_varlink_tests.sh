#!/bin/bash

set -x
if [ ! -n "${PYTHON+ }" ]; then
    if hash python3 > /dev/null 2>&1 /dev/null; then
        PYTHON=$(hash -t python3)
    elif type python3 > /dev/null 2>&1; then
        PYTHON=$(type python3 | awk '{print $3}')
    elif hash python2 > /dev/null 2>&1; then
        PYTHON=$(hash -t python2)
    elif type python2 > /dev/null 2>&1; then
        PYTHON=$(type python2 | awk '{print $3}')
    else
        PYTHON='/usr/bin/python'
    fi
fi

# Create temporary directory for storage
TMPSTORAGE=`mktemp -d`

# Need a location to store the podman socket
mkdir /run/podman

# Run podman in background without systemd for test purposes
bin/podman --storage-driver=vfs --root=${TMPSTORAGE}/crio --runroot=${TMPSTORAGE}/crio-run varlink unix:/run/podman/io.projectatomic.podman&

# Record podman's pid to be killed later
PODMAN_PID=`echo $!`

# Run tests
${PYTHON} -m unittest discover -s test/varlink/

# Kill podman
kill -9 ${PODMAN_PID}

# Clean up
rm -fr ${TMPSTORAGE}
