% podman-top(1)

## NAME
podman\-top - Display the running processes of a container

## SYNOPSIS
**podman top** [*options*] *container* [*format-descriptors*]

**podman container top** [*options*] *container* [*format-descriptors*]

## DESCRIPTION
Display the running processes of the container. The *format-descriptors* are ps (1) compatible AIX format descriptors but extended to print additional information, such as the seccomp mode or the effective capabilities of a given process. The descriptors can either be passed as separated arguments or as a single comma-separated argument. Note that you can also specify options and or flags of ps(1); in this case, Podman will fallback to executing ps with the specified arguments and flags in the container.  Please use the "h*" descriptors if you want to extract host-related information.  For instance, `podman top $name hpid huser` to display the PID and user of the processes in the host context.

## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

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

**stime**

  Process start time (e.g, "2019-12-09 10:50:36 +0100 CET).

## EXAMPLES

By default, `podman-top` prints data similar to `ps -ef`:

```
$ podman top f5a62a71b07
USER   PID   PPID   %CPU    ELAPSED         TTY     TIME   COMMAND
root   1     0      0.000   20.386825206s   pts/0   0s     sh
root   7     1      0.000   16.386882887s   pts/0   0s     sleep
root   8     1      0.000   11.386886562s   pts/0   0s     vi
```

The output can be controlled by specifying format descriptors as arguments after the container:

```
$ podman top -l pid seccomp args %C
PID   SECCOMP   COMMAND     %CPU
1     filter    sh          0.000
8     filter    vi /etc/    0.000
```

Podman will fallback to executing ps(1) in the container if an unknown descriptor is specified.

```
$ podman top -l -- aux
USER   PID   PPID   %CPU    ELAPSED             TTY   TIME   COMMAND
root   1     0      0.000   1h2m12.497061672s   ?     0s     sleep 100000
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **ps(1)**, **seccomp(2)**, **proc(5)**, **capabilities(7)**

## HISTORY
July 2018, Introduce format descriptors by Valentin Rothberg <vrothberg@suse.com>

December 2017, Originally compiled by Brent Baude <bbaude@redhat.com>
