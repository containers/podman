% podman-machine-start(1)

## NAME
podman\-machine\-start - Start a virtual machine

## SYNOPSIS
**podman machine start** *name*

## DESCRIPTION

Starts a virtual machine for Podman.

Podman on MacOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

**podman machine start** starts a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine start myvm
```

## SEE ALSO
podman-machine (1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
