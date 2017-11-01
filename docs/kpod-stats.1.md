% kpod(1) kpod-stats - Display a live stream of 1 or more containers' resource usage statistics
% Ryan Cole
# kpod-stats "1" "July 2017" "kpod"

## NAME
kpod-stats - Display a live stream of 1 or more containers' resource usage statistics

## SYNOPSIS
**kpod** **stats** [*options* [...]] [container]

## DESCRIPTION
Display a live stream of one or more containers' resource usage statistics

## OPTIONS

**--all, -a**

Show all containers.  Only running containers are shown by default

**--no-stream**

Disable streaming stats and only pull the first result, default setting is false

**--format="TEMPLATE"**

Pretty-print images using a Go template


## EXAMPLE

TODO

## SEE ALSO
kpod(1)

## HISTORY
July 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
