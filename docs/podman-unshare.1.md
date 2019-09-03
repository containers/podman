% podman-unshare(1)

## NAME
podman\-unshare - Run a command inside of a modified user namespace

## SYNOPSIS
**podman unshare** [*options*] [*--*] [*command*]

## DESCRIPTION
Launches a process (by default, *$SHELL*) in a new user namespace. The user
namespace is configured so that the invoking user's UID and primary GID appear
to be UID 0 and GID 0, respectively.  Any ranges which match that user and
group in /etc/subuid and /etc/subgid are also mapped in as themselves with the
help of the *newuidmap(1)* and *newgidmap(1)* helpers.

podman unshare is useful for troubleshooting unprivileged operations and for
manually clearing storage and other data related to images and containers.

It is also useful if you want to use the `podman mount` command.  If an unprivileged users wants to mount and work with a container, then they need to execute
podman unshare.  Executing `podman mount` fails for unprivileged users unless the user is running inside a `podman unshare` session.

The unshare session defines two environment variables:

**CONTAINERS_GRAPHROOT** the path to the persistent containers data.
**CONTAINERS_RUNROOT** the path to the volatile containers data.

## EXAMPLE

```
$ podman unshare id
uid=0(root) gid=0(root) groups=0(root),65534(nobody)

$ podman unshare cat /proc/self/uid_map /proc/self/gid_map
         0       1000          1
         1      10000      65536
         0       1000          1
         1      10000      65536
```


## SEE ALSO
podman(1), podman-mount(1), namespaces(7), newuidmap(1), newgidmap(1), user\_namespaces(7)
