% podman-container-exists(1)

## NAME
podman-container-exists - Check if a container exists in local storage

## SYNOPSIS
**podman container exists** [*options*] *container*

## DESCRIPTION
**podman container exists** checks if a container exists in local storage. The **ID** or **Name**
of the container may be used as input.  Podman will return an exit code
of `0` when the container is found.  A `1` will be returned otherwise. An exit code of `125` indicates there
was an issue accessing the local storage.

## OPTIONS

#### **--external**=*true|false*
Check for external containers as well as Podman containers. These external containers are generally created via other container technology such as Buildah or CRI-O.

**-h**, **--help**
Print usage statement

## EXAMPLES

Check if an container called `webclient` exists in local storage (the container does actually exist).
```
$ podman container exists webclient
$ echo $?
0
```

Check if an container called `webbackend` exists in local storage (the container does not actually exist).
```
$ podman container exists webbackend
$ echo $?
1
```

Check if an container called `ubi8-working-container` created via Buildah exists in local storage (the container does not actually exist).
```
$ podman container exists --external ubi8-working-container
$ echo $?
1
```

## SEE ALSO
podman(1)

## HISTORY
November 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
