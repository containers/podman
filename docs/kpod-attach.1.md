% kpod(1) kpod-attach - See the output of pid 1 of a container or enter the container
% Dan Walsh
# kpod-attach "1" "December 2017" "kpod"

## NAME
kpod-attach - Attach to a running container

## SYNOPSIS
**kpod attach [OPTIONS] CONTAINER**

## DESCRIPTION
The attach command allows you to attach to a running container using the container's ID
or name, either to view its ongoing output or to control it interactively.

You can detach from the container (and leave it running) using a configurable key sequence. The default
sequence is CTRL-p CTRL-q. You configure the key sequence using the --detach-keys option

## OPTIONS
**--detach-keys**
Override the key sequence for detaching a container. Format is a single character [a-Z] or
ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _.

**--no-stdin**
Do not attach STDIN. The default is false.

## EXAMPLES ##

```
kpod attach foobar
[root@localhost /]#
```
```
kpod attach 1234
[root@localhost /]#
```
```
kpod attach --no-stdin foobar
```
## SEE ALSO
kpod(1), kpod-exec(1), kpod-run(1)
