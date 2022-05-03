% podman-machine-set(1)

## NAME
podman\-machine\-set - Sets a virtual machine setting

## SYNOPSIS
**podman machine set** [*options*] [*name*]

## DESCRIPTION

Change a machine setting.

## OPTIONS

#### **--cpus**=*number*

Number of CPUs.
Only supported for QEMU machines.

#### **--disk-size**=*number*

Size of the disk for the guest VM in GB.
Can only be increased. Only supported for QEMU machines.

#### **--help**

Print usage statement.

#### **--memory**, **-m**=*number*

Memory (in MB).
Only supported for QEMU machines.

#### **--rootful**=*true|false*

Whether this machine should prefer rootful (`true`) or rootless (`false`)
container execution. This option will also update the current podman
remote connection default if it is currently pointing at the specified
machine name (or `podman-machine-default` if no name is specified).

Unlike [**podman system connection default**](podman-system-connection-default.1.md)
this option will also make the API socket, if available, forward to the rootful/rootless
socket in the VM.

## EXAMPLES

To switch the default VM `podman-machine-default` from rootless to rootful:

```
$ podman machine set --rootful
```

or more explicitly:

```
$ podman machine set --rootful=true
```

To switch the default VM `podman-machine-default` from rootful to rootless:
```
$ podman machine set --rootful=false
```

To switch the VM `myvm` from rootless to rootful:
```
$ podman machine set --rootful myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
February 2022, Originally compiled by Jason Greene <jason.greene@redhat.com>
