% podman-image-exists 1

## NAME
podman-image-exists - Check if an image exists in local storage

## SYNOPSIS
**podman image exists** *image*

## DESCRIPTION
**podman image exists** checks if an image exists in local storage. The **ID** or **Name**
of the image may be used as input.  Podman will return an exit code
of `0` when the image is found.  A `1` will be returned otherwise. An exit code of `125` indicates there
was an issue accessing the local storage.

## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLES

Check if an image called `webclient` exists in local storage (the image does actually exist).
```
$ podman image exists webclient
$ echo $?
0
$
```

Check if an image called `webbackend` exists in local storage (the image does not actually exist).
```
$ podman image exists webbackend
$ echo $?
1
$
```
An example bash script using `podman image exists` to check if the `webbackend` image exists 
in local storage.
```
#!/usr/bin/env bash
set -o errexit

If podman image exists webbackend; then
  echo "webbackend exists"
else
  echo "webbackend does not exist"
fi
echo "The script completed normally"
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-image(1)](podman-image.1.md)**

## HISTORY
November 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
