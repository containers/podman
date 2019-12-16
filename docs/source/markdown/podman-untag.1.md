% podman-untag(1)

## NAME
podman\-untag - Removes one or more names from a locally-stored image

## SYNOPSIS
**podman untag** *image*[:*tag*] [*target-names*[:*tag*]] [*options*]

**podman image untag** *image*[:*tag*] [target-names[:*tag*]] [*options*]

## DESCRIPTION
Removes one or all names of an image. A name refers to the entire image name,
including the optional *tag* after the `:`. If no target image names are
specified, `untag` will remove all tags for the image at once.

## OPTIONS

**--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman untag 0e3bbc2

$ podman untag imageName:latest otherImageName:latest

$ podman untag httpd myregistryhost:5000/fedora/httpd:v2
```


## SEE ALSO
podman(1)

## HISTORY
December 2019, Originally compiled by Sascha Grunert <sgrunert@suse.com>
