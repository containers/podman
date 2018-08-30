% podman-attach(1)

## NAME
podman\-attach - Attach to a running container

## SYNOPSIS
**podman attach** [*options*] *container*

## DESCRIPTION
The attach command allows you to attach to a running container using the container's ID
or name, either to view its ongoing output or to control it interactively.

You can detach from the container (and leave it running) using a configurable key sequence. The default
sequence is CTRL-p CTRL-q. You configure the key sequence using the --detach-keys option

## OPTIONS
**--detach-keys**

Override the key sequence for detaching a container. Format is a single character [a-Z] or
ctrl-[value] where [value] is one of: a-z, @, ^, [, , or _.

**--latest, -l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

**--no-stdin**

Do not attach STDIN. The default is false.

**--sig-proxy**=*true*|*false*

Proxy received signals to the process (non-TTY mode only). SIGCHLD, SIGSTOP, and SIGKILL are not proxied. The default is *true*.

## EXAMPLES ##

```
podman attach foobar
[root@localhost /]#
```
```
podman attach --latest
[root@localhost /]#
```
```
podman attach 1234
[root@localhost /]#
```
```
podman attach --no-stdin foobar
```
## SEE ALSO
podman(1), podman-exec(1), podman-run(1)
