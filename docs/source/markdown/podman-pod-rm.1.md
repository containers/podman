% podman-pod-rm(1)

## NAME
podman\-pod\-rm - Remove one or more stopped pods and containers

## SYNOPSIS
**podman pod rm** [*options*] *pod*

## DESCRIPTION
**podman pod rm** will remove one or more stopped pods and their containers from the host.  The pod name or ID can be used. The \-f option stops all containers and then removes them before removing the pod.

## OPTIONS

#### **--all**, **-a**

Remove all pods.  Can be used in conjunction with \-f as well.

#### **--ignore**, **-i**

Ignore errors when specified pods are not in the container store.  A user might
have decided to manually remove a pod which would lead to a failure during the
ExecStop directive of a systemd service referencing that pod.

#### **--latest**, **-l**

Instead of providing the pod name or ID, remove the last created pod. (This option is not available with the remote Podman client)

#### **--force**, **-f**

Stop running containers and delete all stopped containers before removal of pod.

#### **--pod-id-file**

Read pod ID from the specified file and remove the pod.  Can be specified multiple times.

## EXAMPLE

podman pod rm mywebserverpod

podman pod rm mywebserverpod myflaskserverpod 860a4b23

podman pod rm -f 860a4b23

podman pod rm -f -a

podman pod rm -fa

podman pod rm --pod-id-file /path/to/id/file

## Exit Status
  **0**   All specified pods removed

  **1**   One of the specified pods did not exist, and no other failures

  **2**   One of the specified pods is attached to a container

  **125** The command fails for any other reason

## SEE ALSO
podman-pod(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
