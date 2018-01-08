% podman(1) podman-stats - Display a live stream of 1 or more containers' resource usage statistics
% Ryan Cole
# podman-stats "1" "July 2017" "podman"

## NAME
podman stats - Display a live stream of 1 or more containers' resource usage statistics

## SYNOPSIS
**podman** **stats** [*options* [...]] [container]

## DESCRIPTION
Display a live stream of one or more containers' resource usage statistics

## OPTIONS

**--all, -a**

Show all containers.  Only running containers are shown by default

**--latest, -l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

**--no-reset**

Do not clear the terminal/screen in between reporting intervals

**--no-stream**

Disable streaming stats and only pull the first result, default setting is false

**--format="TEMPLATE"**

Pretty-print images using a Go template


## EXAMPLE

```
# podman stats -a --no-stream

CONTAINER      CPU %   MEM USAGE / LIMIT   MEM %   NET IO    BLOCK IO       PIDS
132ade621b5d   0.00%   1.618MB / 33.08GB   0.00%   0B / 0B   0B / 0B        0
940e00a40a77   0.00%   1.544MB / 33.08GB   0.00%   0B / 0B   0B / 0B        0
72a1dfb44ca7   0.00%   1.528MB / 33.08GB   0.00%   0B / 0B   0B / 0B        0
f5a62a71b07b   0.00%   5.669MB / 33.08GB   0.02%   0B / 0B   0B / 0B        3
31eab2cf93f4   0.00%   16.42MB / 33.08GB   0.05%   0B / 0B   22.43MB / 0B   0

#
```

```
# podman stats --no-stream 31eab2cf93f4
CONTAINER      CPU %   MEM USAGE / LIMIT   MEM %   NET IO    BLOCK IO       PIDS
31eab2cf93f4   0.00%   16.42MB / 33.08GB   0.05%   0B / 0B   22.43MB / 0B   0

#
```
```
# podman stats --no-stream --format=json 31eab2cf93f4
[
    {
        "name": "31eab2cf93f4",
        "id": "31eab2cf93f413af64a3f13d8d78393238658465d75e527333a8577f251162ec",
        "cpu_percent": "0.00%",
        "mem_usage": "16.42MB / 33.08GB",
        "mem_percent": "0.05%",
        "netio": "0B / 0B",
        "blocki": "22.43MB / 0B",
        "pids": 0
    }
]
#
```


## SEE ALSO
podman(1)

## HISTORY
July 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
