% podman(1) podman-kill- Kill one or more containers with a signal
% Brent Baude
# podman-kill"1" "September 2017" "podman"

## NAME
podman kill - Kills one or more containers with a signal

## SYNOPSIS
**podman kill [OPTIONS] CONTAINER [...]**

## DESCRIPTION
The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal.

## OPTIONS

**--signal, s**

Signal to send to the container. For more information on Linux signals, refer to *man signal(7)*.


## EXAMPLE

podman kill mywebserver

podman kill 860a4b23

podman kill --signal TERM 860a4b23

## SEE ALSO
podman(1), podman-stop(1)

## HISTORY
September 2017, Originally compiled by Brent Baude <bbaude@redhat.com>
