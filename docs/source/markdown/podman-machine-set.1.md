% podman-machine-set(1)

## NAME
podman\-machine\-set - Sets a virtual machine setting

## SYNOPSIS
**podman machine set** [*options*] [*name*]

## DESCRIPTION

Sets an updatable virtual machine setting.

Options mirror values passed to `podman machine init`. Only a limited
subset can be changed after machine initialization.

## OPTIONS

#### **--help**

Print usage statement.

#### **--rootfull**=*true|false*

Whether this machine should prefer rootfull (`true`) or rootless (`false`)
container execution. This option will also update the current podman
remote connection default if it is currently pointing at the specified
machine name (or `podman-machine-default` if no name is specified).

Unlike [**podman system connection default**](podman-system-connection-default.1.md)
this option will also make the API socket, if available, forward to the rootfull/rootless
socket in the VM.

## EXAMPLES

To switch the default VM `podman-machine-default` from rootless to rootfull:

```
$ podman machine set --rootfull
```

or more explicitly:

```
$ podman machine set --rootfull=true
```

To switch the default VM `podman-machine-default` from rootfull to rootless:
```
$ podman machine set --rootfull=false
```

To switch the VM `myvm` from rootless to rootfull:
```
$ podman machine set --rootfull myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
February 2022, Originally compiled by Jason Greene <jason.greene@redhat.com>
