% podman-machine-os-apply 1

## NAME
podman\-machine\-os\-apply - Apply an OCI image to a Podman Machine's OS

## SYNOPSIS
**podman machine os apply** [*options*] *image* [vm]

## DESCRIPTION

Apply machine OS changes from an OCI image.

VM's that use OS's that use rpm-ostreee have the capability to rebase itself from the content of an OCI image.
`podman machine image apply` takes an OCI image with container native ostree functionality and rebases itself on that image.

By default, Podman machines on Mac, Linux, and Windows Hyper-V use a customized rpm-ostree based distribution (Fedora CoreOS). Machines based on Microsoft WSL use a
customized Fedora distribution and cannot be updated with this command.

Note: WSL-based machines are upgradable by using the `podman machine ssh <machine_name>` command followed by `sudo dnf update`.  This can, however, result in unexpected results in
Podman client and server version differences.

Podman machine images are stored as OCI images at `quay.io/podman/machine-os`. When applying an image using this
command, the fully qualified OCI reference name must be used including tag where the tag is the
version of Podman that is inside the VM. By default, Podman will attempt to pull only the statement
version as itself.

For more information, see the [rpm-ostree documentation](https://coreos.github.io/rpm-ostree/container/).

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then the OS changes will be applied to `podman-machine-default`.

## OPTIONS

#### **--help**

Print usage statement.

#### **--restart**

Restart VM after applying changes.

## EXAMPLES

Update the default Podman machine to the latest development version of the
bootable OCI image.

Note: This may result in having a newer Podman version inside the machine
than the client.  Unexpected results may occur.

Update the default Podman machine to the most recent Podman 5.4 bootable
OCI image.
```
$ podman machine os apply quay.io/podman/machine-os:5.4
```

Update the specified Podman machine to latest Podman 5.3 bootable OCI image.
```
$ podman machine os apply quay.io/podman/machine-os:5.3 mymachine
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**, **[podman-machine-os(1)](podman-machine-os.1.md)**

## HISTORY
February 2023, Originally compiled by Ashley Cui <acui@redhat.com>
