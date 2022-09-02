% podman-machine-inspect 1

## NAME
podman\-machine\-inspect - Inspect one or more virtual machines

## SYNOPSIS
**podman machine inspect** [*options] *name* ...

## DESCRIPTION

Inspect one or more virtual machines

Obtain greater detail about Podman virtual machines.  More than one virtual machine can be
inspected at once.

Rootless only.

## OPTIONS
#### **--format**

Print results with a Go template.

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine inspect podman-machine-default
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
April 2022, Originally compiled by Brent Baude <bbaude@redhat.com>
