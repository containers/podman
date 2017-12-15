% podman(1) podman-stop - Stop one or more containers
% Brent Baude
# podman-stop "1" "September 2017" "podman"

## NAME
podman stop - Stop one or more containers

## SYNOPSIS
**podman stop [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Stops one or more containers.  You may use container IDs or names as input. The **--timeout** switch
allows you to specify the number of seconds to wait before forcibly stopping the container after the stop command
is issued to the container. The default is 10 seconds.

## OPTIONS

**--timeout, t**

Timeout to wait before forcibly stopping the container

**--all, -a**

Stop all running containers.  This does not include paused containers.


## EXAMPLE

podman stop mywebserver

podman stop 860a4b23

podman stop mywebserver 860a4b23

podman stop --timeout 2 860a4b23

podman stop -a

## SEE ALSO
podman(1), podman-rm(1)

## HISTORY
September 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
