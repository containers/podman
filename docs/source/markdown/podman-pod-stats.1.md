% podman-pod-stats(1)

## NAME
podman\-pod\-stats - Display a live stream of resource usage stats for containers in one or more pods

## SYNOPSIS
**podman pod stats** [*options*] [*pod*]

## DESCRIPTION
Display a live stream of containers in one or more pods resource usage statistics.  Running rootless is only supported on cgroups v2.

## OPTIONS

#### **--all**, **-a**

Show all containers.  Only running containers are shown by default

#### **--latest**, **-l**

Instead of providing the pod name or ID, use the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--no-reset**

Do not clear the terminal/screen in between reporting intervals

#### **--no-stream**

Disable streaming pod stats and only pull the first result, default setting is false

#### **--format**=*template*

Pretty-print container statistics to JSON or using a Go template

Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**    |
| --------------- | ------------------ |
| .Pod            | Pod ID             |
| .CID            | Container ID       |
| .Name           | Container Name     |
| .CPU            | CPU percentage     |
| .MemUsage       | Memory usage       |
| .MemUsageBytes  | Memory usage (IEC) |
| .Mem            | Memory percentage  |
| .NetIO          | Network IO         |
| .BlockIO        | Block IO           |
| .PIDS           | Number of PIDs     |

When using a GO template, you may precede the format with `table` to print headers.
## EXAMPLE

```
# podman pod stats -a --no-stream
ID             NAME              CPU %   MEM USAGE / LIMIT   MEM %   NET IO    BLOCK IO   PIDS
a9f807ffaacd   frosty_hodgkin    --      3.092MB / 16.7GB    0.02%   -- / --   -- / --    2
3b33001239ee   sleepy_stallman   --      -- / --             --      -- / --   -- / --    --
```

```
# podman pod stats --no-stream a9f80
ID             NAME             CPU %   MEM USAGE / LIMIT   MEM %   NET IO    BLOCK IO   PIDS
a9f807ffaacd   frosty_hodgkin   --      3.092MB / 16.7GB    0.02%   -- / --   -- / --    2
```

```
# podman pod stats --no-stream --format=json a9f80
[
    {
        "id": "a9f807ffaacd",
        "name": "frosty_hodgkin",
        "cpu_percent": "--",
        "mem_usage": "3.092MB / 16.7GB",
        "mem_percent": "0.02%",
        "netio": "-- / --",
        "blocki": "-- / --",
        "pids": "2"
    }
]
```

```
# podman pod stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" 6eae
ID             NAME           MEM USAGE / LIMIT
6eae9e25a564   clever_bassi   3.031MB / 16.7GB
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**

## HISTORY
February 2019, Originally compiled by Dan Walsh <dwalsh@redhat.com>
