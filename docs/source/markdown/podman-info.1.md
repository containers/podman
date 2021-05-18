% podman-info(1)

## NAME
podman\-info - Displays Podman related system information

## SYNOPSIS
**podman info** [*options*]

**podman system info** [*options*]

## DESCRIPTION

Displays information pertinent to the host, current storage stats, configured container registries, and build of podman.


## OPTIONS

#### **--debug**, **-D**

Show additional information

#### **--format**=*format*, **-f**

Change output format to "json" or a Go template.


## EXAMPLE

Run podman info with plain text response:
```
$ podman info
host:
  arch: amd64
  buildahVersion: 1.19.0-dev
  cgroupControllers:
  - cpuset
  - cpu
  - io
  - memory
  - pids
  cgroupManager: systemd
  cgroupVersion: v2
  conmon:
    package: conmon-2.0.22-2.fc33.x86_64
    path: /usr/bin/conmon
    version: 'conmon version 2.0.22, commit: 1be6c73605006a85f7ed60b7f76a51e28eb67e01'
  cpus: 8
  distribution:
    distribution: fedora
    version: "33"
  eventLogger: journald
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
  kernel: 5.9.11-200.fc33.x86_64
  linkmode: dynamic
  memFree: 837505024
  memTotal: 16416481280
  ociRuntime:
    name: crun
    package: crun-0.16-1.fc33.x86_64
    path: /usr/bin/crun
    version: |-
      crun version 0.16
      commit: eb0145e5ad4d8207e84a327248af76663d4e50dd
      spec: 1.0.0
      +SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +YAJL
  os: linux
  remoteSocket:
    exists: true
    path: /run/user/3267/podman/podman.sock
  security:
    apparmorEnabled: false
    capabilities: CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT
    rootless: true
    seccompEnabled: true
    selinuxEnabled: true
  slirp4netns:
    executable: /bin/slirp4netns
    package: slirp4netns-1.1.4-4.dev.giteecccdb.fc33.x86_64
    version: |-
      slirp4netns version 1.1.4+dev
      commit: eecccdb96f587b11d7764556ffacfeaffe4b6e11
      libslirp: 4.3.1
      SLIRP_CONFIG_VERSION_MAX: 3
      libseccomp: 2.5.0
  swapFree: 6509203456
  swapTotal: 12591292416
  uptime: 264h 14m 32.73s (Approximately 11.00 days)
registries:
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - registry.centos.org
  - docker.io
store:
  configFile: /home/dwalsh/.config/containers/storage.conf
  containerStore:
    number: 3
    paused: 0
    running: 0
    stopped: 3
  graphDriverName: overlay
  graphOptions:
    overlay.mount_program:
      Executable: /home/dwalsh/bin/fuse-overlayfs
      Package: Unknown
      Version: |-
        fusermount3 version: 3.9.3
        fuse-overlayfs: version 0.7.2
        FUSE library version 3.9.3
        using FUSE kernel interface version 7.31
  graphRoot: /home/dwalsh/.local/share/containers/storage
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "false"
    Supports d_type: "true"
    Using metacopy: "false"
  imageStore:
    number: 77
  runRoot: /run/user/3267/containers
  volumePath: /home/dwalsh/.local/share/containers/storage/volumes
version:
  APIVersion: 3.0.0
  Built: 1608562922
  BuiltTime: Mon Dec 21 10:02:02 2020
  GitCommit: d6925182cdaf94225908a386d02eae8fd3e01123-dirty
  GoVersion: go1.15.5
  OsArch: linux/amd64
  Version: 3.0.0-dev

```
Run podman info with JSON formatted response:
```
{
  "host": {
    "arch": "amd64",
    "buildahVersion": "1.19.0-dev",
    "cgroupManager": "systemd",
    "cgroupVersion": "v2",
    "cgroupControllers": [
      "cpuset",
      "cpu",
      "io",
      "memory",
      "pids"
    ],
    "conmon": {
      "package": "conmon-2.0.22-2.fc33.x86_64",
      "path": "/usr/bin/conmon",
      "version": "conmon version 2.0.22, commit: 1be6c73605006a85f7ed60b7f76a51e28eb67e01"
    },
    "cpus": 8,
    "distribution": {
      "distribution": "fedora",
      "version": "33"
    },
    "eventLogger": "journald",
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
    "kernel": "5.9.11-200.fc33.x86_64",
    "memFree": 894574592,
    "memTotal": 16416481280,
    "ociRuntime": {
      "name": "crun",
      "package": "crun-0.16-1.fc33.x86_64",
      "path": "/usr/bin/crun",
      "version": "crun version 0.16\ncommit: eb0145e5ad4d8207e84a327248af76663d4e50dd\nspec: 1.0.0\n+SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +YAJL"
    },
    "os": "linux",
    "remoteSocket": {
      "path": "/run/user/3267/podman/podman.sock",
      "exists": true
    },
    "security": {
      "apparmorEnabled": false,
      "capabilities": "CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT",
      "rootless": true,
      "seccompEnabled": true,
      "selinuxEnabled": true
    },
    "slirp4netns": {
      "executable": "/bin/slirp4netns",
      "package": "slirp4netns-1.1.4-4.dev.giteecccdb.fc33.x86_64",
      "version": "slirp4netns version 1.1.4+dev\ncommit: eecccdb96f587b11d7764556ffacfeaffe4b6e11\nlibslirp: 4.3.1\nSLIRP_CONFIG_VERSION_MAX: 3\nlibseccomp: 2.5.0"
    },
    "swapFree": 6509203456,
    "swapTotal": 12591292416,
    "uptime": "264h 13m 12.39s (Approximately 11.00 days)",
    "linkmode": "dynamic"
  },
  "store": {
    "configFile": "/home/dwalsh/.config/containers/storage.conf",
    "containerStore": {
      "number": 3,
      "paused": 0,
      "running": 0,
      "stopped": 3
    },
    "graphDriverName": "overlay",
    "graphOptions": {
      "overlay.mount_program": {
  "Executable": "/home/dwalsh/bin/fuse-overlayfs",
  "Package": "Unknown",
  "Version": "fusermount3 version: 3.9.3\nfuse-overlayfs: version 0.7.2\nFUSE library version 3.9.3\nusing FUSE kernel interface version 7.31"
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
      "number": 77
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
    "APIVersion": "3.0.0",
    "Version": "3.0.0-dev",
    "GoVersion": "go1.15.5",
    "GitCommit": "d6925182cdaf94225908a386d02eae8fd3e01123-dirty",
    "BuiltTime": "Mon Dec 21 10:02:02 2020",
    "Built": 1608562922,
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
