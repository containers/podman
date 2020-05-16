% podman-info(1)

## NAME
podman\-info - Displays Podman related system information

## SYNOPSIS
**podman info** [*options*]

**podman system info** [*options*]

## DESCRIPTION

Displays information pertinent to the host, current storage stats, configured container registries, and build of podman.


## OPTIONS

**-D**, **--debug**

Show additional information

**-f**, **--format**=*format*

Change output format to "json" or a Go template.


## EXAMPLE

Run podman info with plain text response:
```
$ podman info
host:
  arch: amd64
  buildahVersion: 1.15.0
  cgroupVersion: v1
  conmon:
    package: conmon-2.0.16-2.fc32.x86_64
    path: /usr/bin/conmon
    version: 'conmon version 2.0.16, commit: 1044176f7dd177c100779d1c63931d6022e419bd'
  cpus: 8
  distribution:
    distribution: fedora
    version: "32"
  eventLogger: file
  hostname: localhost.localdomain
  idMappings:
    gidmap:
    - container_id: 0
      host_id: 3267
      size: 1
    - container_id: 1
      host_id: 100000
      size: 65536
    uidmap:
    - container_id: 0
      host_id: 3267
      size: 1
    - container_id: 1
      host_id: 100000
      size: 65536
  kernel: 5.6.11-300.fc32.x86_64
  linkmode: dynamic
  memFree: 1401929728
  memTotal: 16416161792
  ociRuntime:
    name: runc
    package: containerd.io-1.2.10-3.2.fc31.x86_64
    path: /usr/bin/runc
    version: |-
      runc version 1.0.0-rc8+dev
      commit: 3e425f80a8c931f88e6d94a8c831b9d5aa481657
      spec: 1.0.1-dev
  os: linux
  rootless: true
  slirp4netns:
    executable: /bin/slirp4netns
    package: slirp4netns-1.0.0-1.fc32.x86_64
    version: |-
      slirp4netns version 1.0.0
      commit: a3be729152a33e692cd28b52f664defbf2e7810a
      libslirp: 4.2.0
  swapFree: 8291610624
  swapTotal: 8296329216
  uptime: 52h 29m 39.78s (Approximately 2.17 days)
registries:
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - registry.centos.org
  - docker.io
store:
  configFile: /home/dwalsh/.config/containers/storage.conf
  containerStore:
    number: 2
    paused: 0
    running: 0
    stopped: 2
  graphDriverName: overlay
  graphOptions:
    overlay.mount_program:
      Executable: /home/dwalsh/bin/fuse-overlayfs
      Package: Unknown
      Version: |-
        fusermount3 version: 3.9.1
        fuse-overlayfs: version 0.7.2
        FUSE library version 3.9.1
        using FUSE kernel interface version 7.31
  graphRoot: /home/dwalsh/.local/share/containers/storage
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "false"
    Supports d_type: "true"
    Using metacopy: "false"
  imageStore:
    number: 7
  runRoot: /run/user/3267/containers
  volumePath: /home/dwalsh/.local/share/containers/storage/volumes
version:
  Built: 1589899246
  BuiltTime: Tue May 19 10:40:46 2020
  GitCommit: c3678ce3289f4195f3f16802411e795c6a587c9f-dirty
  GoVersion: go1.14.2
  OsArch: linux/amd64
  RemoteAPIVersion: 1
  Version: 2.0.0
```
Run podman info with JSON formatted response:
```
{
  "host": {
    "arch": "amd64",
    "buildahVersion": "1.15.0",
    "cgroupVersion": "v1",
    "conmon": {
      "package": "conmon-2.0.16-2.fc32.x86_64",
      "path": "/usr/bin/conmon",
      "version": "conmon version 2.0.16, commit: 1044176f7dd177c100779d1c63931d6022e419bd"
    },
    "cpus": 8,
    "distribution": {
      "distribution": "fedora",
      "version": "32"
    },
    "eventLogger": "file",
    "hostname": "localhost.localdomain",
    "idMappings": {
      "gidmap": [
        {
          "container_id": 0,
          "host_id": 3267,
          "size": 1
        },
        {
          "container_id": 1,
          "host_id": 100000,
          "size": 65536
        }
      ],
      "uidmap": [
        {
          "container_id": 0,
          "host_id": 3267,
          "size": 1
        },
        {
          "container_id": 1,
          "host_id": 100000,
          "size": 65536
        }
      ]
    },
    "kernel": "5.6.11-300.fc32.x86_64",
    "memFree": 1380356096,
    "memTotal": 16416161792,
    "ociRuntime": {
      "name": "runc",
      "package": "containerd.io-1.2.10-3.2.fc31.x86_64",
      "path": "/usr/bin/runc",
      "version": "runc version 1.0.0-rc8+dev\ncommit: 3e425f80a8c931f88e6d94a8c831b9d5aa481657\nspec: 1.0.1-dev"
    },
    "os": "linux",
    "rootless": true,
    "slirp4netns": {
      "executable": "/bin/slirp4netns",
      "package": "slirp4netns-1.0.0-1.fc32.x86_64",
      "version": "slirp4netns version 1.0.0\ncommit: a3be729152a33e692cd28b52f664defbf2e7810a\nlibslirp: 4.2.0"
    },
    "swapFree": 8291610624,
    "swapTotal": 8296329216,
    "uptime": "52h 27m 39.38s (Approximately 2.17 days)",
    "linkmode": "dynamic"
  },
  "store": {
    "configFile": "/home/dwalsh/.config/containers/storage.conf",
    "containerStore": {
      "number": 2,
      "paused": 0,
      "running": 0,
      "stopped": 2
    },
    "graphDriverName": "overlay",
    "graphOptions": {
      "overlay.mount_program": {
  "Executable": "/home/dwalsh/bin/fuse-overlayfs",
  "Package": "Unknown",
  "Version": "fusermount3 version: 3.9.1\nfuse-overlayfs: version 0.7.2\nFUSE library version 3.9.1\nusing FUSE kernel interface version 7.31"
}
    },
    "graphRoot": "/home/dwalsh/.local/share/containers/storage",
    "graphStatus": {
      "Backing Filesystem": "extfs",
      "Native Overlay Diff": "false",
      "Supports d_type": "true",
      "Using metacopy": "false"
    },
    "imageStore": {
      "number": 7
    },
    "runRoot": "/run/user/3267/containers",
    "volumePath": "/home/dwalsh/.local/share/containers/storage/volumes"
  },
  "registries": {
    "search": [
  "registry.fedoraproject.org",
  "registry.access.redhat.com",
  "registry.centos.org",
  "docker.io"
]
  },
  "version": {
    "RemoteAPIVersion": 1,
    "Version": "2.0.0",
    "GoVersion": "go1.14.2",
    "GitCommit": "c3678ce3289f4195f3f16802411e795c6a587c9f-dirty",
    "BuiltTime": "Tue May 19 10:40:46 2020",
    "Built": 1589899246,
    "OsArch": "linux/amd64"
  }
}
```
Run podman info and only get the registries information.
```
$ podman info --format={{".Registries"}}
map[registries:[docker.io quay.io registry.fedoraproject.org registry.access.redhat.com]]
```

## SEE ALSO
podman(1), containers-registries.conf(5), containers-storage.conf(5)
