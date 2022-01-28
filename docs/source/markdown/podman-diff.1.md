% podman-diff(1)

## NAME
podman\-diff - Inspect changes on a container or image's filesystem

## SYNOPSIS
**podman diff** [*options*] *container|image* [*container|image*]

## DESCRIPTION
Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer or the second argument when given.

The output is prefixed with the following symbols:

| Symbol | Description |
|--------|-------------|
| A | A file or directory was added.   |
| D | A file or directory was deleted. |
| C | A file or directory was changed. |

## OPTIONS

#### **--format**

Alter the output into a different format.  The only valid format for **podman diff** is `json`.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

```
$ podman diff container1
A /myscript.sh
```

```
$ podman diff --format json myimage
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

```
$ podman diff container1 image1
A /test
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-container-diff(1)](podman-container-diff.1.md)**, **[podman-image-diff(1)](podman-image-diff.1.md)**

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
