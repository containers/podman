% podman-pod-start "1"

## NAME
podman\-pod\-start - Start one or more containers

## SYNOPSIS
**podman pod start**  *pod* ...

## DESCRIPTION
Start one or more pods.  You may use pod IDs or names as input. The pod must have a container attached
to be started.

## EXAMPLE

podman pod start mywebserverpod

podman pod start 860a4b23 5421ab4


## SEE ALSO
podman-pod(1), podman-pod-create(1)

## HISTORY
November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
July 2018, Adapted from podman start man page by Peter Hunt <pehunt@redhat.com>
