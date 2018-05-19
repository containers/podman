% podman(1) podman-varlink - Waits on a container
% Brent Baude
# podman-varlink "1" "April 2018" "podman"

## NAME
podman\-varlink - Runs the varlink backend interface

## SYNOPSIS
**podman varlink**
[**--help**|**-h**]
VARLINK_URI

## DESCRIPTION
Starts the varlink service that allows varlink clients to interact with podman.
<!--
More will go here as the docs and api firm up.
-->

**podman [GLOBAL OPTIONS] varlink **

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

## EXAMPLES

  podman varlink unix:/run/podman/io.projectatomic.podman
<!--
    TODO: More examples with TCP can be added when that works
    as well.
-->

## CONFIGURATION

Users of the podman varlink service should enable the  io.projectatomic.podman.socket and io.projectatomic.podman.service.

You can do this via systemctl

systemctl enable --now io.projectatomic.podman.socket

## SEE ALSO
podman(1), systemctl(1)

## HISTORY
April 2018, Originally compiled by Brent Baude<bbaude@redhat.com>
