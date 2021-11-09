% podman-machine-start(1)

## NAME
podman\-machine\-start - Start a virtual machine

## SYNOPSIS
**podman machine start** [*name*]

## DESCRIPTION

Starts a virtual machine for Podman.

Podman on macOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

Only one Podman managed VM can be active at a time. If a VM is already running,
`podman machine start` will return an error.

**podman machine start** starts a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine start myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
