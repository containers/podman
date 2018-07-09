% podman-pod-rm "1"

## NAME
podman\-pod\-rm - Remove one or more pods

## SYNOPSIS
**podman rm** [*options*] *container*

## DESCRIPTION
**podman pod rm** will remove one or more pods from the host.  The pod name or ID can be used. The -f option stops all containers then removes them before removing the pod. Without the -f option, a pod cannot be removed if it has attached containers.

## OPTIONS

**--force, f**

Stop running containers and delete all stopped containers before removal of pod.

**--all, a**

Remove all pods.  Can be used in conjunction with -f and -r as well.

## EXAMPLE

podman pod rm mywebserverpod

podman pod rm mywebserverpod myflaskserverpod 860a4b23

podman pod rm -f 860a4b23

podman pod rm -f -a

podman pod rm -fa

## SEE ALSO
podman-pod(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
July 2018, Adapted from podman rm man page by Peter Hunt <pehunt@redhat.com>
