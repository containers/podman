% podman-machine-init(1)

## NAME
podman\-machine\-init - Initialize a new virtual machine

## SYNOPSIS
**podman machine init** [*options*] [*name*]

## DESCRIPTION

Initialize a new virtual machine for Podman.

Podman on macOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel.

**podman machine init** initializes a new Linux virtual machine where containers are run.
SSH keys are automatically generated to access the VM, and system connections to the root account
and a user account inside the VM are added.

By default, the VM distribution is [Fedora CoreOS](https://getfedora.org/en/coreos?stream=testing).
Fedora CoreOS upgrades come out every 14 days and are detected and installed automatically. The VM will be rebooted during the upgrade.
For more information on updates and advanced configuration, please see the FCOS update docs [here](https://docs.fedoraproject.org/en-US/fedora-coreos/auto-updates/) and [here](https://coreos.github.io/zincati/usage/updates-strategy/).

## OPTIONS

#### **--cpus**=*number*

Number of CPUs.

#### **--disk-size**=*number*

Size of the disk for the guest VM in GB.

#### **--ignition-override-path**

A path to a complete or partial ignition file that will be merged with the ignition
file created with `podman init`.  The merging of fields and values are to add them rather
than replace the original entries. More than one override may be specified.

#### **--ignition-path**

Fully qualified path of the ignition file.

If an ignition file is provided, the file
will be copied into the user's CONF_DIR and renamed.  Additionally, no SSH keys will
be generated nor will a system connection be made.  It is assumed that the user will
do these things manually or handle otherwise.

#### **--image-path**

Fully qualified path or URL to the VM image.
Can also be set to `testing`, `next`, or `stable` to pull down default image.
Defaults to `testing`.

#### **--memory**, **-m**=*number*

Memory (in MB).

#### **--now**

Start the virtual machine immediately after it has been initialized.

#### **--timezone**

Set the timezone for the machine and containers.  Valid values are `local` or
a `timezone` such as `America/Chicago`.  A value of `local`, which is the default,
means to use the timezone of the machine host.

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine init
$ podman machine init myvm
$ podman machine init --disk-size 50
$ podman machine init --memory=1024 myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
