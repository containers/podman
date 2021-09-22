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

## OPTIONS

#### **--cpus**=*number*

Number of CPUs.

#### **--disk-size**=*number*

Size of the disk for the guest VM in GB.

#### **--ignition-path**

Fully qualified path of the ignition file.

If an ignition file is provided, the file
will be copied into the user's CONF_DIR and renamed.  Additionally, no SSH keys will
be generated nor will a system connection be made.  It is assumed that the user will
do these things manually or handle otherwise.

#### **--image-path**

Fully qualified path or URL to the VM image.
Can also be set to `testing` or `stable` to pull down default image.
Defaults to `testing`.

#### **--memory**, **-m**=*number*

Memory (in MB).

#### **--now**

Start the virtual machine immediately after it has been initialized.

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
podman-machine(1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
