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

#### **--rootful**=*true|false*

Whether this machine should prefer rootful (`true`) or rootless (`false`)
container execution. This option will also update the current podman
remote connection default if it is currently pointing at the specified
machine name (or `podman-machine-default` if no name is specified).

API forwarding, if available, will follow this setting.

#### **--type**=*provider name*

The type of virtualization provider. It allows using a virtualization technology or provider different from the system. By default, the system provider (QEMU for Linux and macOS systems, and WSL for Windows) will be used.

#### **--help**

Print usage statement.

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
