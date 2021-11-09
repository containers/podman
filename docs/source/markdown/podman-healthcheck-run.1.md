% podman-healthcheck-run(1)

## NAME
podman\-healthcheck\-run - Run a container healthcheck

## SYNOPSIS
**podman healthcheck run** *container*

## DESCRIPTION

Runs the healthcheck command defined in a running container manually.  The resulting error codes are defined
as follows:

* 0 = healthcheck command succeeded
* 1 = healthcheck command failed
* 125 = an error has occurred

Possible errors that can occur during the healthcheck are:
* unable to find the container
* container has no defined healthcheck
* container is not running

## OPTIONS
#### **--help**

Print usage statement


## EXAMPLES

```
$ podman healthcheck run mywebapp
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-healthcheck(1)](podman-healthcheck.1.md)**

## HISTORY
Feb 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
