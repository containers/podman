% podman-machine-ssh(1)

## NAME
podman\-machine\-ssh - SSH into a virtual machine

## SYNOPSIS
**podman machine ssh** [*options*] [*name*] [*command* [*arg* ...]]

## DESCRIPTION

SSH into a Podman-managed virtual machine.

Podman on MacOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

## OPTIONS

#### **--execute**, **-e**

Execute the given command on the VM

#### **--help**

Print usage statement.

## EXAMPLES

To get an interactive session with a VM called `myvm`:
```
$ podman machine ssh myvm
```

To run a command on a VM called `myvm`:
```
$ podman machine ssh -e myvm -- rpm -q podman
```

## SEE ALSO
podman-machine (1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
