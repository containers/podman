% PODMAN(1) Podman Man Pages
% Brent Baude
% November 2018
# NAME
podman-container-exists- Check if a container exists in local storage

# SYNOPSIS
**podman container exists**
[**-h**|**--help**]
CONTAINER

# DESCRIPTION
**podman container exists** checks if a container exists in local storage. The **ID** or **Name**
of the container may be used as input.  Podman will return an exit code
of `0` when the container is found.  A `1` will be returned otherwise. An exit code of `125` indicates there
was an issue accessing the local storage.

## Examples ##

Check if an container called `webclient` exists in local storage (the container does actually exist).
```
$ sudo podman container exists webclient
$ echo $?
0
$
```

Check if an container called `webbackend` exists in local storage (the container does not actually exist).
```
$ sudo podman container exists webbackend
$ echo $?
1
$
```

## SEE ALSO
podman(1)

# HISTORY
November 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
