% podman-quadlet-rm 1

## NAME
podman\-quadlet\-rm - Removes an installed quadlet

## SYNOPSIS
**podman quadlet rm** [*options*] *quadlet-name* [*quadlet-name*]...

## DESCRIPTION

Remove one or more installed Quadlets from the current user.

## OPTIONS

#### **--all**, **-a**

Remove all Quadlets for the current user.

#### **--force**, **-f**

Remove running quadlets.

#### **--ignore**, **-i**

Do not error for Quadlets that do not exist.

#### **--reload-systemd**

Reload systemd after removal

## EXAMPLES

```
$ podman quadlet rm myquadlet.container
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
