% podman-version "1"

## NAME
podman\-info - Display system information


## SYNOPSIS
**podman info** [*options*]


## DESCRIPTION

Displays information pertinent to the host, current storage stats, configured container registries, and build of podman.


## OPTIONS

**--debug, -D**

Show additional information

**--format**

Change output format to "json" or a Go template.


## EXAMPLE

```
$ podman info
host:
  MemFree: 7822168064
  MemTotal: 33080606720
  SwapFree: 34357637120
  SwapTotal: 34359734272
  arch: amd64
  cpus: 8
  hostname: localhost.localdomain
  kernel: 4.13.16-300.fc27.x86_64
  os: linux
  uptime: 142h 13m 55.64s (Approximately 5.92 days)
insecure registries:
  registries: []
registries:
  registries:
  - docker.io
  - registry.fedoraproject.org
  - registry.access.redhat.com
store:
  ContainerStore:
    number: 7
  GraphDriverName: overlay
  GraphOptions:
  - overlay.override_kernel_check=true
  GraphRoot: /var/lib/containers/storage
  GraphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
  ImageStore:
    number: 10
  RunRoot: /var/run/containers/storage
```
```
$ podman info --debug --format json
{
    "host": {
        "MemFree": 7506157568,
        "MemTotal": 33080606720,
        "SwapFree": 34357637120,
        "SwapTotal": 34359734272,
        "arch": "amd64",
        "cpus": 8,
        "hostname": "localhost.localdomain",
        "kernel": "4.13.16-300.fc27.x86_64",
        "os": "linux",
        "uptime": "142h 17m 17.04s (Approximately 5.92 days)"

        ... removed for brevity

        "ImageStore": {
            "number": 10
        },
        "RunRoot": "/var/run/containers/storage"
    }
}

```

```
$ podman info --format={{".registries"}}
map[registries:[docker.io registry.fedoraproject.org registry.access.redhat.com]]
```

## SEE ALSO
podman(1), registries.conf(5), storage.conf(5), crio(8)
