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

  podman varlink unix:/run/io.projectatomic.podman
<!--
    TODO: More examples with TCP can be added when that works
    as well.
-->

## SEE ALSO
podman(1)

## HISTORY
April 2018, Originally compiled by Brent Baude<bbaude@redhat.com>
