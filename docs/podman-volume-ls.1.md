% podman-volume-ls(1)

## NAME
podman\-volume\-ls - List volumes

## SYNOPSIS
**podman volume ls** [*options*]

## DESCRIPTION

Lists all the volumes that exist. The output can be filtered using the **--filter**
flag and can be formatted to either JSON or a Go template using the **--format**
flag. Use the **--quiet** flag to print only the volume names.

## OPTIONS

**--filter**=""

Filter volume output.

**--format**=""

Format volume output using Go template.

**--help**

Print usage statement.

**-q**, **--quiet**=[]

Print volume output in quiet mode. Only print the volume names.

## EXAMPLES

```
$ podman volume ls

$ podman volume ls --format json

$ podman volume ls --format "{{.Driver}} {{.Scope}}"

$ podman volume ls --filter name=foo,label=blue
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
