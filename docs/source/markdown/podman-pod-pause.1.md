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

Instead of providing the pod name or ID, pause the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

podman pod pause mywebserverpod

podman pod pause 860a4b23

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-pod-unpause(1)](podman-pod-unpause.1.md)**, **[podman-pause(1)](podman-pause.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
