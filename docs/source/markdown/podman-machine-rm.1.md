% podman-machine-rm(1)

## NAME
podman\-machine\-rm - Remove a virtual machine

## SYNOPSIS
**podman machine rm** [*options*] [*name*]

## DESCRIPTION

Remove a virtual machine and its related files.  What is actually deleted
depends on the virtual machine type.  For all virtual machines, the generated
SSH keys and the podman system connection are deleted.  The ignition files
generated for that VM are also removed as is its image file on the filesystem.

Users get a display of what will be deleted and are required to confirm unless the option `--force`
is used.


## OPTIONS

#### **--help**

Print usage statement.

#### **--force**

Delete without confirmation

#### **--save-ignition**

Do not delete the generated ignition file

#### **--save-image**

Do not delete the VM image

#### **--save-keys**

Do not delete the SSH keys for the VM.  The system connection is always
deleted.

#### **--type**=*provider name*

The type of virtualization provider. It allows using a virtualization technology or provider different from the system. By default, the system provider (QEMU for Linux and macOS systems, and WSL for Windows) will be used.

## EXAMPLES

Remove a VM named "test1"

```
$ podman machine rm test1

The following files will be deleted:

/home/user/.ssh/test1
/home/user/.ssh/test1.pub
/home/user/.config/containers/podman/machine/qemu/test1.ign
/home/user/.local/share/containers/podman/machine/qemu/test1_fedora-coreos-33.20210315.1.0-qemu.x86_64.qcow2
/home/user/.config/containers/podman/machine/qemu/test1.json

Are you sure you want to continue? [y/N] y
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
