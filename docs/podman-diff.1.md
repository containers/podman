% podman(1) podman-diff - Inspect changes on a container or image's filesystem
% Dan Walsh
# podman-diff "1" "August 2017" "podman"

## NAME
podman diff - Inspect changes on a container or image's filesystem

## SYNOPSIS
**podman** **diff** [*options* [...]] NAME

## DESCRIPTION
Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer

## OPTIONS

**--format**

Alter the output into a different format.  The only valid format for diff is `json`.


## EXAMPLE

podman diff redis:alpine
C /usr
C /usr/local
C /usr/local/bin
A /usr/local/bin/docker-entrypoint.sh

podman diff --format json redis:alpine
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
podman(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
