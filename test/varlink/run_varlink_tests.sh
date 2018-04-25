#!/bin/bash

if [ ! -n "${PYTHON+ }" ]; then
  if hash python3 > /dev/null 2>&1; then
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
TMPSTORAGE=`mktemp -d /tmp/podman.XXXXXXXXXX`
trap 'rm -fr ${TMPSTORAGE}' EXIT

export PODMAN_HOST="unix:${TMPSTORAGE}/podman/io.projectatomic.podman"

# Need a location to store the podman socket
mkdir -p ${TMPSTORAGE}/podman

systemd-cat -t podman -p notice bin/podman --version

set -x
# Run podman in background without systemd for test purposes
systemd-cat -t podman -p notice \
  bin/podman --storage-driver=vfs --root=${TMPSTORAGE}/crio \
  --runroot=${TMPSTORAGE}/crio-run varlink ${PODMAN_HOST} &

${PYTHON} -m unittest discover -s test/varlink/ $@
pkill podman
