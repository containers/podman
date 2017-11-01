% kpod(1) kpod-unpause - Unpause one or more containers
% Dan Walsh
# kpod-unpause "1" "September 2017" "kpod"

## NAME
kpod unpause - Unpause one or more containers

## SYNOPSIS
**kpod unpause [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Unpauses the processes in one or more containers.  You may use container IDs or names as input.

## EXAMPLE

kpod unpause mywebserver

kpod unpause 860a4b23

## SEE ALSO
kpod(1), kpod-pause(1)

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
