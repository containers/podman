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

#### **--help**

Print usage statement.

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

#### **--rootful**=*true|false*

Whether this machine should prefer rootful (`true`) or rootless (`false`)
container execution. This option will also determine the remote connection default
if there is no existing remote connection configurations.

API forwarding, if available, will follow this setting.

#### **--timezone**

Set the timezone for the machine and containers.  Valid values are `local` or
a `timezone` such as `America/Chicago`.  A value of `local`, which is the default,
means to use the timezone of the machine host.

#### **--volume**, **-v**=*source:target*

Mounts a volume from source to target.

Create a mount. If /host-dir:/machine-dir is specified as the `*source:target*`,
Podman mounts _host-dir_ in the host to _machine-dir_ in the Podman machine.

The root filesystem is mounted read-only in the default operating system,
so mounts must be created under the /mnt directory.

Default volume mounts are defined in *containers.conf*.  Unless changed, the default values
is `$HOME:$HOME`.

#### **--volume-driver**

Driver to use for mounting volumes from the host, such as `virtfs`.

## EXAMPLES

```
$ podman machine init
$ podman machine init myvm
$ podman machine init --rootful
$ podman machine init --disk-size 50
$ podman machine init --memory=1024 myvm
$ podman machine init -v /Users:/mnt/Users
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
