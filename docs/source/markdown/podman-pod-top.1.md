% podman-pod-top 1

## NAME
podman\-pod\-top - Display the running processes of containers in a pod

## SYNOPSIS
**podman pod top** [*options*] *pod* [*format-descriptors*]

## DESCRIPTION
Display the running processes of containers in a pod. The *format-descriptors* are ps (1) compatible AIX format descriptors but extended to print additional information, such as the seccomp mode or the effective capabilities of a given process. The descriptors can either be passed as separated arguments or as a single comma-separated argument. Note that you can specify options and/or additionally options of ps(1); in this case, Podman will fallback to executing ps with the specified arguments and options in the container.

## OPTIONS

#### **--help**, **-h**

  Print usage statement

#### **--latest**, **-l**

Instead of providing the pod name or ID, use the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## FORMAT DESCRIPTORS

Please refer to podman-top(1) for a full list of available descriptors.

## EXAMPLES

By default, `podman-pod-top` prints data similar to `ps -ef`:

```
$ podman pod top b031293491cc
USER   PID   PPID   %CPU    ELAPSED             TTY   TIME   COMMAND
root   1     0      0.000   2h5m38.737137571s   ?     0s     top
root   8     0      0.000   2h5m15.737228361s   ?     0s     top
```

The output can be controlled by specifying format descriptors as arguments after the pod:

```
$ podman pod top -l pid seccomp args %C
PID   SECCOMP   COMMAND   %CPU
1     filter    top       0.000
1     filter    /bin/sh   0.000
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **ps(1)**, **seccomp(2)**, **proc(5)**, **capabilities(7)**

## HISTORY
August 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
