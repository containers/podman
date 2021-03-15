% podman-machine-ssh(1)

## NAME
podman\-machine\-ssh - SSH into a virtual machine

## SYNOPSIS
**podman machine ssh** *name*

## DESCRIPTION

SSH into a Podman-managed virtual machine.

Podman on MacOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine ssh myvm
```

## SEE ALSO
podman-machine (1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
