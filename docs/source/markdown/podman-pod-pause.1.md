% podman-pod-pause(1)

## NAME
podman\-pod\-pause - Pause one or more pods

## SYNOPSIS
**podman pod pause** [*options*] *pod* ...

## DESCRIPTION
Pauses all the running processes in the containers of one or more pods.  You may use pod IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Pause all pods.

#### **--latest**, **-l**

Instead of providing the pod name or ID, pause the last created pod. (This option is not available with the remote Podman client)

## EXAMPLE

podman pod pause mywebserverpod

podman pod pause 860a4b23

## SEE ALSO
podman-pod(1), podman-pod-unpause(1), podman-pause(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
