% podman-tag 1

## NAME
podman\-tag - Add an additional name to a local image

## SYNOPSIS
**podman tag** *image*[:*tag*] [*target-name*[:*tag*]...] [*options*]

**podman image tag** *image*[:*tag*] [*target-name*[:*tag*]...] [*options*]

## DESCRIPTION
Assigns a new image name to an existing image.  A full name refers to the entire
image name, including the optional *tag* after the `:`.  If there is no *tag*
provided, then Podman defaults to `latest` for both the *image* and the
*target-name*.

## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman tag 0e3bbc2 fedora:latest

$ podman tag httpd myregistryhost:5000/fedora/httpd:v2
```


## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
December 2019, Update description to refer to 'name' instead of 'alias' by Sascha Grunert <sgrunert@suse.com>
July 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
