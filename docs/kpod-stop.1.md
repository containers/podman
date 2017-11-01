% kpod(1) kpod-stop - Stop one or more containers
% Brent Baude
# kpod-stop "1" "September 2017" "kpod"

## NAME
kpod stop - Stop one or more containers

## SYNOPSIS
**kpod stop [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Stops one or more containers.  You may use container IDs or names as input. The **--timeout** switch
allows you to specify the number of seconds to wait before forcibly stopping the container after the stop command
is issued to the container. The default is 10 seconds.

## OPTIONS

**--timeout, t**

Timeout to wait before forcibly stopping the container


## EXAMPLE

kpod stop mywebserver

kpod stop 860a4b23

kpod stop --timeout 2 860a4b23

## SEE ALSO
kpod(1), kpod-rm(1)

## HISTORY
September 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
