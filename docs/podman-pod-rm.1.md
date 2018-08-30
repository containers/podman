% podman-pod-rm(1)

## NAME
podman\-pod\-rm - Remove one or more pods

## SYNOPSIS
**podman pod rm** [*options*] *pod*

## DESCRIPTION
**podman pod rm** will remove one or more pods from the host.  The pod name or ID can be used. The \-f option stops all containers and then removes them before removing the pod. Without the \-f option, a pod cannot be removed if it has associated containers.

## OPTIONS

**--all, a**

Remove all pods.  Can be used in conjunction with \-f as well.

**--latest, -l**

Instead of providing the pod name or ID, remove the last created pod.

**--force, f**

Stop running containers and delete all stopped containers before removal of pod.

## EXAMPLE

podman pod rm mywebserverpod

podman pod rm mywebserverpod myflaskserverpod 860a4b23

podman pod rm -f 860a4b23

podman pod rm -f -a

podman pod rm -fa

## SEE ALSO
podman-pod(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
