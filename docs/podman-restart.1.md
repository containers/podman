% podman-restart "1"

## NAME
podman\-restart - Restart a container

## SYNOPSIS
**podman attach [OPTIONS] CONTAINER [CONTAINER...]**

## DESCRIPTION
The restart command allows containers to be restarted using their ID or name.
Containers will be stopped if they are running and then restarted. Stopped
containers will not be stopped and will only be started.

## OPTIONS
**--timeout**

Timeout to wait before forcibly stopping the container

**--latest, -l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

## EXAMPLES ##

```
podman restart -l
ec588fc80b05e19d3006bf2e8aa325f0a2e2ff1f609b7afb39176ca8e3e13467
```

```
podman restart ff6cf1
ff6cf1e5e77e6dba1efc7f3fcdb20e8b89ad8947bc0518be1fcb2c78681f226f
```

```
podman restart --timeout 4 test1 test2
c3bb026838c30e5097f079fa365c9a4769d52e1017588278fa00d5c68ebc1502
17e13a63081a995136f907024bcfe50ff532917988a152da229db9d894c5a9ec
```

## SEE ALSO
podman(1), podman-run(1), podman-start(1), podman-create(1)

## HISTORY
March 2018, Originally compiled by Matt Heon <mheon@redhat.com>
