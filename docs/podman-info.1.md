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
  Conmon:
    package: conmon-1.10.3-1.gite558bd5.fc28.x86_64
    path: /usr/libexec/crio/conmon
    version: 'conmon version 1.10.3, commit: 55022fb3be4382a61599b7024a677f9a642ae0a7'
  MemFree: 2428579840
  MemTotal: 16679260160
  OCIRuntime:
    package: runc-1.0.0-46.dev.gitb4e2ecb.fc28.x86_64
    path: /usr/bin/runc
    version: 'runc version spec: 1.0.0'
  SwapFree: 0
  SwapTotal: 0
  arch: amd64
  cpus: 4
  hostname: localhost.localdomain
  kernel: 4.17.11-200.fc28.x86_64
  os: linux
  uptime: 23h 16m 57.86s (Approximately 0.96 days)
insecure registries:
  registries: []
registries:
  registries:
  - docker.io
  - quay.io
  - registry.fedoraproject.org
  - registry.access.redhat.com
store:
  ContainerStore:
    number: 3
  GraphDriverName: overlay
  GraphOptions:
  - overlay.mountopt=nodev
  GraphRoot: /var/lib/containers/storage
  GraphStatus:
    Backing Filesystem: xfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
  ImageStore:
    number: 2
  RunRoot: /var/run/containers/storage
```
Run podman info with JSON formatted response:
```
$ podman info --debug --format json
{
    "debug": {
        "compiler": "gc",
        "git commit": "",
        "go version": "go1.10",
        "podman version": "0.8.2-dev"
    },
    "host": {
        "Conmon": {
            "package": "conmon-1.10.3-1.gite558bd5.fc28.x86_64",
            "path": "/usr/libexec/crio/conmon",
            "version": "conmon version 1.10.3, commit: 55022fb3be4382a61599b7024a677f9a642ae0a7"
        },
        "MemFree": 2484420608,
        "MemTotal": 16679260160,
        "OCIRuntime": {
            "package": "runc-1.0.0-46.dev.gitb4e2ecb.fc28.x86_64",
            "path": "/usr/bin/runc",
            "version": "runc version spec: 1.0.0"
        },
        "SwapFree": 0,
        "SwapTotal": 0,
        "arch": "amd64",
        "cpus": 4,
        "hostname": "localhost.localdomain",
        "kernel": "4.17.11-200.fc28.x86_64",
        "os": "linux",
        "uptime": "23h 14m 45.48s (Approximately 0.96 days)"
    },
    "insecure registries": {
        "registries": []
    },
    "registries": {
        "registries": [
            "docker.io",
            "quay.io",
            "registry.fedoraproject.org",
            "registry.access.redhat.com"
        ]
    },
    "store": {
        "ContainerStore": {
            "number": 3
        },
        "GraphDriverName": "overlay",
        "GraphOptions": [
            "overlay.mountopt=nodev"
        ],
        "GraphRoot": "/var/lib/containers/storage",
        "GraphStatus": {
            "Backing Filesystem": "xfs",
            "Native Overlay Diff": "true",
            "Supports d_type": "true"
        },
        "ImageStore": {
            "number": 2
        },
        "RunRoot": "/var/run/containers/storage"
    }
}
	```
Run podman info and only get the registries information.
```
$ podman info --format={{".registries"}}
map[registries:[docker.io quay.io registry.fedoraproject.org registry.access.redhat.com]]
```

## SEE ALSO
podman(1), registries.conf(5), storage.conf(5), crio(8)
