% podman-manifest-add(1)

## NAME
podman\-manifest\-add - Add an image to a manifest list or image index

## SYNOPSIS
**podman manifest add** *listnameorindexname* *imagename*

## DESCRIPTION

Adds the specified image to the specified manifest list or image index.

## RETURN VALUE
The list image's ID.

## OPTIONS

**--all**

If the image which should be added to the list or index is itself a list or
index, add all of the contents to the local list.  By default, only one image
from such a list or index will be added to the list or index.  Combining
*--all* with any of the other options described below is NOT recommended.

**--annotation** *annotation=value*

Set an annotation on the entry for the newly-added image.

**--arch**

Override the architecture which the list or index records as a requirement for
the image.  If *imageName* refers to a manifest list or image index, the
architecture information will be retrieved from it.  Otherwise, it will be
retrieved from the image's configuration information.

**--features**

Specify the features list which the list or index records as requirements for
the image.  This option is rarely used.

**--os**

Override the OS which the list or index records as a requirement for the image.
If *imagename* refers to a manifest list or image index, the OS information
will be retrieved from it.  Otherwise, it will be retrieved from the image's
configuration information.

**--os-version**

Specify the OS version which the list or index records as a requirement for the
image.  This option is rarely used.

**--variant**

Specify the variant which the list or index records for the image.  This option
is typically used to distinguish between multiple entries which share the same
architecture value, but which expect different versions of its instruction set.

## EXAMPLE

```
podman manifest add mylist:v1.11 docker://fedora
71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

```
podman manifest add --all mylist:v1.11 docker://fedora
71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

```
podman manifest add --arch arm64 --variant v8 mylist:v1.11 docker://71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

## SEE ALSO
podman(1), podman-manifest(1), podman-manifest-create(1), podman-manifest-inspect(1), podman-rmi(1)
