% kpod(1) kpod-rm - Remove one or more containers
% Ryan Cole
# kpod-rm "1" "August 2017" "kpod"

## NAME
kpod rm - Remove one or more containers

## SYNOPSIS
**kpod** **rm** [*options* [...]] container

## DESCRIPTION
Kpod rm will remove one or more containers from the host.  The container name or ID can be used.  This does not remove images.  Running containers will not be removed without the -f option

## OPTIONS

**--force, f**

Force the removal of a running container


## EXAMPLE

kpod rm mywebserver

kpod rm -f 860a4b23

## SEE ALSO
kpod(1), kpod-rmi(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
