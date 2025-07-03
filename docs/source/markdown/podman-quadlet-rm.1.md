% podman-quadlet-rm 1

## NAME
podman\-quadlet\-rm - Removes an installed quadlet

## SYNOPSIS
**podman quadlet rm** [*options*] *quadlet* [*quadlet*]...

## DESCRIPTION

Remove one or more installed Quadlets from the current user. Following command also takes application name
as input and removes all the Quadlets which belongs to that specific application.

Note: If a quadlet is part of an application, removing that specific quadlet will remove the entire application.
When a quadlet is installed from a directory, all files installed from that directory—including both quadlet and non-quadlet files—are considered part
of a single application.

## OPTIONS

#### **--all**, **-a**

Remove all Quadlets for the current user.

#### **--force**, **-f**

Remove running quadlets.

#### **--ignore**, **-i**

Do not error for Quadlets that do not exist.

#### **--reload-systemd**

Reload systemd after removing Quadlets (default true).
In order to disable it users need to manually set the value
of this flag to `false`.

## EXAMPLES

```
$ podman quadlet rm myquadlet.container
myquadlet.container
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
