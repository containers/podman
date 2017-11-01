% kpod(1) kpod-pause - Pause one or more containers
% Dan Walsh
# kpod-pause "1" "September 2017" "kpod"

## NAME
kpod pause - Pause one or more containers

## SYNOPSIS
**kpod pause [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Pauses all the processes in one or more containers.  You may use container IDs or names as input.

## EXAMPLE

kpod pause mywebserver

kpod pause 860a4b23

## SEE ALSO
kpod(1), kpod-unpause(1)

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
