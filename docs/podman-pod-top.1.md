% podman-pod-top(1)

## NAME
podman\-pod\-top - Display the running processes of containers in a pod

## SYNOPSIS
**podman top** [*options*] *pod* [*format-descriptors*]

## DESCRIPTION
Display the running process of containers in a pod. The *format-descriptors* are ps (1) compatible AIX format descriptors but extended to print additional information, such as the seccomp mode or the effective capabilities of a given process.

## OPTIONS

**--help, -h**

  Print usage statement

**--latest, -l**

Instead of providing the pod name or ID, use the last created pod.

## FORMAT DESCRIPTORS

The following descriptors are supported in addition to the AIX format descriptors mentioned in ps (1):

**args, capbnd, capeff, capinh, capprm, comm, etime, group, hgroup, hpid, huser, label, nice, pcpu, pgid, pid, ppid, rgroup, ruser, seccomp, state, time, tty, user, vsz**

**capbnd**

  Set of bounding capabilities. See capabilities (7) for more information.

**capeff**

  Set of effective capabilities. See capabilities (7) for more information.

**capinh**

  Set of inheritable capabilities. See capabilities (7) for more information.

**capprm**

  Set of permitted capabilities. See capabilities (7) for more information.

**hgroup**

  The corresponding effective group of a container process on the host.

**hpid**

  The corresponding host PID of a container process.

**huser**

  The corresponding effective user of a container process on the host.

**label**

  Current security attributes of the process.

**seccomp**

  Seccomp mode of the process (i.e., disabled, strict or filter). See seccomp (2) for more information.

**state**

  Process state codes (e.g, **R** for *running*, **S** for *sleeping*). See proc(5) for more information.

## EXAMPLES

By default, `podman-top` prints data similar to `ps -ef`:

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
podman-pod(1), ps(1), seccomp(2), proc(5), capabilities(7)

## HISTORY
August 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
