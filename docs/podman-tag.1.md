% podman-tag "1"

## NAME
podman\-tag - Add an additional name to a local image

## SYNOPSIS
**podman tag** *image*[:*tag*] *target-name*[:*tag*]
[**--help**|**-h**]

## DESCRIPTION
Assigns a new alias to an image.  An alias refers to the entire image name, including the optional
*tag* after the `:`.  If you do not provide *tag*, podman will default to `latest` for both
the *image* and the *target-name*.

## OPTIONS

**--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman tag 0e3bbc2 fedora:latest

$ podman tag httpd myregistryhost:5000/fedora/httpd:v2
```


## SEE ALSO
podman(1), crio(8)

## HISTORY
July 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
