% podman-machine-start 1

## NAME
podman\-machine\-start - Start a virtual machine

## SYNOPSIS
**podman machine start** [*name*]

## DESCRIPTION

Starts a virtual machine for Podman.

Rootless only.

Podman on MacOS and Windows requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel. Podman machine must be used to manage MacOS and Windows machines,
but can be optionally used on Linux.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be started.

Only one Podman managed VM can be active at a time. If a VM is already running,
`podman machine start` returns an error.

**podman machine start** starts a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

#### **--no-info**

Suppress informational tips.

#### **--quiet**, **-q**

Suppress machine starting status output.

## EXAMPLES

```
$ podman machine start myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
