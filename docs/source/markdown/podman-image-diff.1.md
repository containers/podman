% podman-image-diff(1)

## NAME
podman-image-diff - Inspect changes on an image's filesystem

## SYNOPSIS
**podman image diff** [*options*] *name*

## DESCRIPTION
Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer

## OPTIONS

**--format**

Alter the output into a different format.  The only valid format for diff is `json`.

## EXAMPLE

```
# podman diff redis:old redis:alpine
C /usr
C /usr/local
C /usr/local/bin
A /usr/local/bin/docker-entrypoint.sh
```

```
# podman diff --format json redis:old redis:alpine
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
```

## SEE ALSO
podman(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
