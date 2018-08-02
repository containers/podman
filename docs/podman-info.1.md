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

Run podman info with plain text response:
```
$ podman info
host:
  MemFree: 28464242688
  MemTotal: 33147686912
  OCIRuntimeVersion: 'runc version spec: 1.0.0'
  SwapFree: 34359734272
  SwapTotal: 34359734272
  arch: amd64
  conmonVersion: 'conmon version 1.11.0-dev, commit: 42209340c7abcab66f47e9161fa0f1b16ea8c134'
  cpus: 8
  hostname: localhost.localdomain
  kernel: 4.17.9-200.fc28.x86_64
  os: linux
  uptime: 47m 34.95s
insecure registries:
  registries: []
registries:
  registries:
  - docker.io
  - registry.fedoraproject.org
  - registry.access.redhat.com
store:
  ContainerStore:
    number: 40
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
Run podman info with JSON formatted response:
```
$ podman info --debug --format json
{
    "host": {
        "MemFree": 28421324800,
        "MemTotal": 33147686912,
        "OCIRuntimeVersion": "runc version spec: 1.0.0",
        "SwapFree": 34359734272,
        "SwapTotal": 34359734272,
        "arch": "amd64",
        "conmonVersion": "conmon version 1.11.0-dev, commit: 42209340c7abcab66f47e9161fa0f1b16ea8c134",
        "cpus": 8,
        "hostname": "localhost.localdomain",
        "kernel": "4.17.9-200.fc28.x86_64",
        "os": "linux",
        "uptime": "50m 20.27s"
    },
    "insecure registries": {
        "registries": []
    },
    "registries": {
        "registries": [
            "docker.io",
            "registry.fedoraproject.org",
            "registry.access.redhat.com",
        ]
    },
    "store": {
        "ContainerStore": {
            "number": 40
        },
        "GraphDriverName": "overlay",
        "GraphOptions": [
            "overlay.override_kernel_check=true"
        ],
        "GraphRoot": "/var/lib/containers/storage",
        "GraphStatus": {
            "Backing Filesystem": "extfs",
            "Native Overlay Diff": "true",
            "Supports d_type": "true"
        },
        "ImageStore": {
            "number": 10
        },
        "RunRoot": "/var/run/containers/storage"
    }
}
```
Run podman info and only get the registries information.
```
$ podman info --format={{".registries"}}
map[registries:[docker.io registry.fedoraproject.org registry.access.redhat.com]]
```

## SEE ALSO
podman(1), registries.conf(5), storage.conf(5), crio(8)
