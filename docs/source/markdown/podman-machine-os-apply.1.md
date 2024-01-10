% podman-machine-os-apply 1

## NAME
podman\-machine\-os\-apply - Apply an OCI image to a Podman Machine's OS

## SYNOPSIS
**podman machine os apply** [*options*] *image* [vm]

## DESCRIPTION

Apply machine OS changes from an OCI image.

VM's that use OS's that use rpm-ostreee have the capability to rebase itself from the content of an OCI image.
`podman machine image apply` takes an OCI image with container native ostree functionality and rebases itself on that image.

By default, Podman machines on Mac and Linux use an rpm-ostree based distribution (Fedora CoreOS).

For more information, see the [rpm-ostree documentation](https://coreos.github.io/rpm-ostree/container/).

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then the OS changes will be applied to `podman-machine-default`.

## OPTIONS

#### **--help**

Print usage statement.

#### **--restart**

Restart VM after applying changes.

## EXAMPLES

Update the default Podman machine to the specified bootable OCI image.
```
$ podman machine os apply quay.io/podman_next
```

Update the specified Podman machine to the specified bootable OCI image.
```
$ podman machine os apply quay.io/podman_next podman-machine-default
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**, **[podman-machine-os(1)](podman-machine-os.1.md)**

## HISTORY
February 2023, Originally compiled by Ashley Cui <acui@redhat.com>
