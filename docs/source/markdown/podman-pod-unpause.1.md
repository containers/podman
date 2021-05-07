% podman-pod-unpause(1)

## NAME
podman\-pod\-unpause - Unpause one or more pods

## SYNOPSIS
**podman pod unpause** [*options*] *pod* ...

## DESCRIPTION
Unpauses all the paused processes in the containers of one or more pods.  You may use pod IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Unpause all pods.

#### **--latest**, **-l**

Instead of providing the pod name or ID, unpause the last created pod. (This option is not available with the remote Podman client)

## EXAMPLE

podman pod unpause mywebserverpod

podman pod unpause 860a4b23

## SEE ALSO
podman-pod(1), podman-pod-pause(1), podman-unpause(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
