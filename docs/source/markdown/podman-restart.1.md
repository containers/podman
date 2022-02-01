% podman-restart(1)

## NAME
podman\-restart - Restart one or more containers

## SYNOPSIS
**podman restart** [*options*] *container* ...

**podman container restart** [*options*] *container* ...

## DESCRIPTION
The restart command allows containers to be restarted using their ID or name.
Containers will be stopped if they are running and then restarted. Stopped
containers will not be stopped and will only be started.

## OPTIONS
#### **--all**, **-a**
Restart all containers regardless of their current state.

#### **--latest**, **-l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--running**
Restart all containers that are already in the *running* state.

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the container.

## EXAMPLES

Restart the latest container
```
$ podman restart -l
ec588fc80b05e19d3006bf2e8aa325f0a2e2ff1f609b7afb39176ca8e3e13467
```

Restart a specific container by partial container ID
```
$ podman restart ff6cf1
ff6cf1e5e77e6dba1efc7f3fcdb20e8b89ad8947bc0518be1fcb2c78681f226f
```

Restart two containers by name with a timeout of 4 seconds
```
$ podman restart --time 4 test1 test2
c3bb026838c30e5097f079fa365c9a4769d52e1017588278fa00d5c68ebc1502
17e13a63081a995136f907024bcfe50ff532917988a152da229db9d894c5a9ec
```

Restart all running containers
```
$ podman restart --running
```

Restart all containers
```
$ podman restart --all
```

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
March 2018, Originally compiled by Matt Heon <mheon@redhat.com>
