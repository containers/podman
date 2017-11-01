% kpod(1) kpod-rename - Rename a container
% Ryan Cole
# kpod-images "1" "March 2017" "kpod"

## NAME
kpod rename - Rename a container

## SYNOPSIS
**kpod** **rename** CONTAINER NEW-NAME

## DESCRIPTION
Rename a container.  Container may be created, running, paused, or stopped

## EXAMPLE

kpod rename redis-container webserver

kpod rename a236b9a4 mycontainer

## SEE ALSO
kpod(1)

## HISTORY
March 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
