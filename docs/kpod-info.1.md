% kpod(1) kpod-version - Simple tool to view version information
% Vincent Batts
% kpod-version "1" "JULY 2017" "kpod"

## NAME
kpod-info - Display system information


## SYNOPSIS
**kpod** **info** [*options* [...]]


## DESCRIPTION

Information display here pertain to the host, current storage stats, and build of kpod. Useful for the user and when reporting issues.


## OPTIONS

**--debug, -D**

Show additional information

**--format**

Change output format to "json" or a Go template.


## EXAMPLE

`kpod info`

`kpod info --debug --format json| jq .host.kernel`

## SEE ALSO
crio(8), crio.conf(5)
