% kpod(1) kpod-top - display the running processes of a container
% Brent Baude

## NAME
kpod top - Display the running processes of a container

## SYNOPSIS
**kpod top**
[**--help**|**-h**]

## DESCRIPTION
Display the running process of the container. ps-OPTION can be any of the options you would pass to a Linux ps command

**kpod [GLOBAL OPTIONS] top [OPTIONS]**

## OPTIONS

**--help, -h**
  Print usage statement

**--format**
  Display the output in an alternate format.  The only supported format is **JSON**.

## EXAMPLES

```
# kpod top f5a62a71b07
  UID   PID  PPID %CPU STIME TT           TIME CMD
    0 18715 18705  0.0 10:35 pts/0    00:00:00 /bin/bash
    0 18741 18715  0.0 10:35 pts/0    00:00:00 vi
#
```

```
#kpod --log-level=debug top f5a62a71b07 -o fuser,f,comm,label
FUSER    F COMMAND         LABEL
root     4 bash            system_u:system_r:container_t:s0:c429,c1016
root     0 vi              system_u:system_r:container_t:s0:c429,c1016
#
```
```
# kpod top --format=json f5a62a71b07b -o %cpu,%mem,command,blocked
[
    {
        "CPU": "0.0",
        "MEM": "0.0",
        "COMMAND": "vi",
        "BLOCKED": "0000000000000000",
        "START": "",
        "TIME": "",
        "C": "",
        "CAUGHT": "",
        ...
```
## SEE ALSO
kpod(1), ps(1)

## HISTORY
December 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
