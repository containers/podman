% podman-wait(1)

## NAME
podman\-wait - Wait on one or more containers to stop and print their exit codes

## SYNOPSIS
**podman wait** [*options*] *container* [...]

**podman container wait** [*options*] *container* [...]

## DESCRIPTION
Waits on one or more containers to stop.  The container can be referred to by its
name or ID.  In the case of multiple containers, Podman will wait on each consecutively.
After all specified containers are stopped, the containers' return codes are printed
separated by newline in the same order as they were given to the command.

## OPTIONS

#### **--condition**=*state*
Condition to wait on (default "stopped")

#### **--help**, **-h**

 Print usage statement

#### **--interval**, **-i**=*duration*
  Time interval to wait before polling for completion. A duration string is a sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h". Time unit defaults to "ms".

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)


## EXAMPLES

```
$ podman wait mywebserver
0

$ podman wait --latest
0

$ podman wait --interval 2s
0

$ podman wait 860a4b23
1

$ podman wait mywebserver myftpserver
0
125
```

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
September 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
