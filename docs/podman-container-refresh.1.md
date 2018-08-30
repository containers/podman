% podman-container-refresh(1)

## NAME
podman\-container\-refresh - Refresh all containers

## SYNOPSIS
**podman container refresh**

## DESCRIPTION
The refresh command refreshes the state of all containers to pick up database
schema or general configuration changes. It is not necessary during normal
operation, and will typically be invoked by package managers after finishing an
upgrade of the Podman package.

As part of refresh, all running containers will be restarted.

## EXAMPLES ##

```
podman container refresh
[root@localhost /]#
```

## SEE ALSO
podman(1), podman-run(1)
