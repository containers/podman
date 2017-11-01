% kpod(1) kpod-wait - Waits on a container
% Brent Baude
# kpod-wait "1" "September 2017" "kpod"

## NAME
kpod wait - Waits on one or more containers to stop and prints exit code

## SYNOPSIS
**kpod wait**
[**--help**|**-h**]

## DESCRIPTION
Waits on one or more containers to stop.  The container can be referred to by its
name or ID.  In the case of multiple containers, kpod will wait on each consecutively.
After the container stops, the container's return code is printed.

**kpod [GLOBAL OPTIONS] wait **

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

## EXAMPLES

  kpod wait mywebserver

  kpod wait 860a4b23

  kpod wait mywebserver myftpserver

## SEE ALSO
kpod(1), crio(8), crio.conf(5)

## HISTORY
September 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
