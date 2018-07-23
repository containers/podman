% podman-top "1"

## NAME
podman\-top - Display the running processes of a container

## SYNOPSIS
**podman top** [*options*] *container* [*format-descriptors*]

## DESCRIPTION
Display the running process of the container. The *format-descriptors* are ps (1) compatible AIX format descriptors but extended to print additional information, such as the seccomp mode or the effective capabilities of a given process.

## OPTIONS

**--help, -h**

  Print usage statement

**--latest, -l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

## FORMAT DESCRIPTORS

The following descriptors are supported in addition to the AIX format descriptors mentioned in ps (1):

**capinh**

  Set of inheritable capabilities. See capabilities (7) for more information.

**capprm**

  Set of permitted capabilities. See capabilities (7) for more information.

**capeff**

  Set of effective capabilities. See capabilities (7) for more information.

**capbnd**

  Set of bounding capabilities. See capabilities (7) for more information.

**seccomp**

  Seccomp mode of the process (i.e., disabled, strict or filter). See seccomp (2) for more information.

**label**

  Current security attributes of the process.

## EXAMPLES

By default, `podman-top` prints data similar to `ps -ef`:

```
# podman top f5a62a71b07
USER   PID   PPID   %CPU    ELAPSED         TTY     TIME   COMMAND
root   1     0      0.000   20.386825206s   pts/0   0s     sh
root   7     1      0.000   16.386882887s   pts/0   0s     sleep
root   8     1      0.000   11.386886562s   pts/0   0s     vi
```

The output can be controlled by specifying format descriptors as arguments after the container:

```
# sudo ./bin/podman top -l pid seccomp args %C
PID   SECCOMP   COMMAND     %CPU
1     filter    sh          0.000
8     filter    vi /etc/    0.000
```

## SEE ALSO
podman(1), ps(1), seccomp(2), capabilities(7)

## HISTORY
December 2017, Originally compiled by Brent Baude <bbaude@redhat.com>

July 2018, Introduce format descriptors by Valentin Rothberg <vrothberg@suse.com>
