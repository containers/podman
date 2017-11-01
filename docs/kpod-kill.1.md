% kpod(1) kpod-kill- Kill one or more containers with a signal
% Brent Baude
# kpod-kill"1" "September 2017" "kpod"

## NAME
kpod kill - Kills one or more containers with a signal

## SYNOPSIS
**kpod kill [OPTIONS] CONTAINER [...]**

## DESCRIPTION
The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal.

## OPTIONS

**--signal, s**

Signal to send to the container. For more information on Linux signals, refer to *man signal(7)*.


## EXAMPLE

kpod kill mywebserver

kpod kill 860a4b23

kpod kill --signal TERM 860a4b23

## SEE ALSO
kpod(1), kpod-stop(1)

## HISTORY
September 2017, Originally compiled by Brent Baude <bbaude@redhat.com>
