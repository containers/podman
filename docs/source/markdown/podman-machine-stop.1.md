% podman-machine-stop(1)

## NAME
podman\-machine\-stop - Stop a virtual machine

## SYNOPSIS
**podman machine stop** [*name*]

## DESCRIPTION

Stops a virtual machine.

Podman on macOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

**podman machine stop** stops a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine stop myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
