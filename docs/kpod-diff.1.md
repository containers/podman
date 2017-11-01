% kpod(1) kpod-diff - Inspect changes on a container or image's filesystem
% Dan Walsh
# kpod-diff "1" "August 2017" "kpod"

## NAME
kpod diff - Inspect changes on a container or image's filesystem

## SYNOPSIS
**kpod** **diff** [*options* [...]] NAME

## DESCRIPTION
Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer

## OPTIONS

**--format**

Alter the output into a different format.  The only valid format for diff is `json`.


## EXAMPLE

kpod diff redis:alpine
C /usr
C /usr/local
C /usr/local/bin
A /usr/local/bin/docker-entrypoint.sh

kpod diff --format json redis:alpine
{
  "changed": [
    "/usr",
    "/usr/local",
    "/usr/local/bin"
  ],
  "added": [
    "/usr/local/bin/docker-entrypoint.sh"
  ]
}

## SEE ALSO
kpod(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
