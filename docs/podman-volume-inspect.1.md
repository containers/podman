% podman-volume-inspect(1)

## NAME
podman\-volume\-inspect - Inspect one or more volumes

## SYNOPSIS
**podman volume inspect** [*options*]

## DESCRIPTION

Display detailed information on one or more volumes. The output can be formated using
the **--format** flag and a Go template. To get detailed information about all the
existing volumes, use the **--all** flag.


## OPTIONS

**-a**, **--all**=""

Inspect all volumes.

**--format**=""

Format volume output using Go template

**--help**

Print usage statement


## EXAMPLES

```
$ podman volume inspect myvol

$ podman volume inspect --all

$ podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
