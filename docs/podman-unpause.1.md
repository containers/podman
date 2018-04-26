% podman(1) podman-unpause - Unpause one or more containers
% Dan Walsh
# podman-unpause "1" "September 2017" "podman"

## NAME
podman\-unpause - Unpause one or more containers

## SYNOPSIS
**podman unpause [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Unpauses the processes in one or more containers.  You may use container IDs or names as input.

## EXAMPLE

podman unpause mywebserver

podman unpause 860a4b23

## SEE ALSO
podman(1), podman-pause(1)

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
