% podman-image-diff(1)

## NAME

podman-image-diff - Inspect changes on an image's filesystem

## SYNOPSIS

**podman image diff** [*options*] *name*

## DESCRIPTION

**podman image diff** displays changes on an image's filesystem.  The image will be compared to its parent layer.

The output is prefixed with the following symbols:

| Symbol | Description |
|--------|-------------|
| A | A file or directory was added. |
| D | A file or directory was deleted. |
| C | A file or directory was changed. |

## OPTIONS

#### **--format**, **-f**=*format*

Alter the output into a different format.  The only valid format for diff is `JSON`.

## EXAMPLE

- Diff of two different python images.

```
# podman diff python:latest python:alpine
C /usr
C /usr/local
C /usr/local/bin
A /usr/local/bin/docker-entrypoint.sh
```

- `JSON` formatted diff of two different redis images. The returned `JSON` object contains arrays specifying changes in the filesystem. 

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

**[podman(1)](podman.1.md)**

## HISTORY

August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
