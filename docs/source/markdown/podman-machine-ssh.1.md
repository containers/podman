% podman-machine-ssh(1)

## NAME
podman\-machine\-ssh - SSH into a virtual machine

## SYNOPSIS
**podman machine ssh** [*name*] [*command* [*arg* ...]]

## DESCRIPTION

SSH into a Podman-managed virtual machine and optionally execute a command
on the virtual machine.  Unless using the default virtual machine, the
first argument must be the virtual machine name. The optional command to
execute can then follow. If no command is provided, an interactive session
with the virtual machine is established.


## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

To get an interactive session with the default virtual machine:

```
$ podman machine ssh
```

To get an interactive session with a VM called `myvm`:
```
$ podman machine ssh myvm
```

To run a command on the default virtual machine:
```
$ podman machine ssh rpm -q podman
```

To run a command on a VM called `myvm`:
```
$ podman machine ssh  myvm rpm -q podman
```

## SEE ALSO
podman-machine (1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
