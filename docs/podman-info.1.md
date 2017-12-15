% podman(1) podman-version - Simple tool to view version information
% Vincent Batts
% podman-version "1" "JULY 2017" "podman"

## NAME
podman-info - Display system information


## SYNOPSIS
**podman** **info** [*options* [...]]


## DESCRIPTION

Information display here pertain to the host, current storage stats, and build of podman. Useful for the user and when reporting issues.


## OPTIONS

**--debug, -D**

Show additional information

**--format**

Change output format to "json" or a Go template.


## EXAMPLE

`podman info`

`podman info --debug --format json| jq .host.kernel`

## SEE ALSO
crio(8), crio.conf(5)
