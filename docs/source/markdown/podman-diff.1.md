% podman-diff(1)

## NAME
podman\-diff - Inspect changes on a container or image's filesystem

## SYNOPSIS
**podman diff** [*options*] *name*

**podman container diff** [*options*] *name*

## DESCRIPTION
Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer

## OPTIONS

#### **--format**

Alter the output into a different format.  The only valid format for diff is `json`.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client)

## EXAMPLE

```
# podman diff redis:alpine
C /usr
C /usr/local
C /usr/local/bin
A /usr/local/bin/docker-entrypoint.sh
```

```
# podman diff --format json redis:alpine
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
