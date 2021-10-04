#!/usr/bin/env bash
# test_podman_pods.sh
# A script to be run at the command line with Podman installed.
# This should be run against a new kit to provide base level testing
# on a freshly installed machine with no images or container in
# play.  This currently needs to be run as root.
#
#
# To run this command:
#
# /bin/bash -v test_podman_baseline.sh -e # Stop on error
# /bin/bash -v test_podman_baseline.sh    # Continue on error
#

set -x

# This scripts needs the jq json parser
if [ -z $(command -v jq2) ]; then
    echo "This script requires the jq parser"
    exit 1
fi

# process input args
stoponerror=0
while getopts "den" opt; do
    case "$opt" in
    e) stoponerror=1
       ;;
    esac
done


if [ "$stoponerror" -eq 1 ]
then
    echo "Script will stop on unexpected errors."
    set -e
    trap "Failed test ..." ERR
fi


########
# Create a named and unnamed pod
########
podman pod create --name foobar
podid=$(podman pod create)

########
# Delete a named and unnamed pod
########
podman pod rm foobar
podman pod rm $podid

########
# Create a named pod and run a container in it
########
podman pod create --name foobar
ctrid=$(podman run --pod foobar -dt docker.io/library/alpine:latest top)
podman ps --no-trunc | grep $ctrid

########
# Containers in a pod share network namespace
########
podman run -dt --pod foobar docker.io/library/nginx:latest
podman run -it --rm --pod foobar registry.fedoraproject.org/fedora-minimal:29 curl http://localhost

########
# There should be 3 containers running now
########
let numContainers=$(podman pod ps --format json | jq -r '.[0] .numberOfContainers')
[ $numContainers -eq 3 ]

########
# Pause a container in a pod
########
podman pause $ctrid
[ $(podman ps -a -f status=paused --format json | jq -r '.[0] .id') == $ctrid ]

########
# Unpause a container in a pod
########
podman unpause $ctrid
podman ps  -q --no-trunc | grep $ctrid

########
# Stop a pod and its containers
########
podman pod stop foobar
[ $(podman inspect $ctrid | jq -r '.[0] .State .Running') == "false" ]

########
# Start a pod and its containers
########
podman pod start foobar
podman run -it --rm --pod foobar registry.fedoraproject.org/fedora-minimal:29 curl http://localhost

########
# Pause a pod and its containers
########
podman pod pause foobar
[ $(podman pod ps --format json | jq -r '.[0] .status') == "Paused" ]

########
# Unpause a pod and its containers
########
podman pod unpause foobar
podman run -it --rm --pod foobar registry.fedoraproject.org/fedora-minimal:29 curl http://localhost

########
# Kill a pod and its containers
########
podman pod kill foobar
[ $(podman inspect $ctrid | jq -r '.[0] .State .Running') == "false" ]

########
# Remove all pods and their containers
########
podman pod rm -t 0 -fa
