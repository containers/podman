% podman(1) podman-pause - Pause one or more containers
% Dan Walsh
# podman-pause "1" "September 2017" "podman"

## NAME
podman\-pause - Pause one or more containers

## SYNOPSIS
**podman pause [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Pauses all the processes in one or more containers.  You may use container IDs or names as input.

## EXAMPLE

podman pause mywebserver

podman pause 860a4b23

## SEE ALSO
podman(1), podman-unpause(1)

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
