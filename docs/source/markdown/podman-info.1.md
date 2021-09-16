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
  buildahVersion: 1.22.3
  cgroupControllers: []
  cgroupManager: systemd
  cgroupVersion: v2
  conmon:
    package: conmon-2.0.29-2.fc34.x86_64
    path: /usr/bin/conmon
    version: 'conmon version 2.0.29, commit: '
  cpus: 8
  distribution:
    distribution: fedora
    variant: workstation
    version: "34"
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
  kernel: 5.13.13-200.fc34.x86_64
  linkmode: dynamic
  logDriver: journald
  memFree: 1351262208
  memTotal: 16401895424
  ociRuntime:
    name: crun
    package: crun-1.0-1.fc34.x86_64
    path: /usr/bin/crun
    version: |-
      crun version 1.0
      commit: 139dc6971e2f1d931af520188763e984d6cdfbf8
      spec: 1.0.0
      +SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +YAJL
  os: linux
  remoteSocket:
    path: /run/user/3267/podman/podman.sock
  security:
    apparmorEnabled: false
    capabilities: CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT
    rootless: true
    seccompEnabled: true
    seccompProfilePath: /usr/share/containers/seccomp.json
    selinuxEnabled: true
  serviceIsRemote: false
  slirp4netns:
    executable: /bin/slirp4netns
    package: slirp4netns-1.1.12-2.fc34.x86_64
    version: |-
      slirp4netns version 1.1.12
      commit: 7a104a101aa3278a2152351a082a6df71f57c9a3
      libslirp: 4.4.0
      SLIRP_CONFIG_VERSION_MAX: 3
      libseccomp: 2.5.0
  swapFree: 16818888704
  swapTotal: 16886259712
  uptime: 33h 57m 32.85s (Approximately 1.38 days)
plugins:
  log:
  - k8s-file
  - none
  - journald
  network:
  - bridge
  - macvlan
  volume:
  - local
registries:
  localhost:5000:
    Blocked: false
    Insecure: true
    Location: localhost:5000
    MirrorByDigestOnly: false
    Mirrors: null
    Prefix: localhost:5000
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - docker.io
store:
  configFile: /home/dwalsh/.config/containers/storage.conf
  containerStore:
    number: 2
    paused: 0
    running: 1
    stopped: 1
  graphDriverName: overlay
  graphOptions: {}
  graphRoot: /home/dwalsh/.local/share/containers/storage
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
    Using metacopy: "false"
  imageStore:
    number: 37
  runRoot: /run/user/3267/containers
  volumePath: /home/dwalsh/.local/share/containers/storage/volumes
version:
  APIVersion: 3.3.1
  Built: 1631137208
  BuiltTime: Wed Sep  8 17:40:08 2021
  GitCommit: ab272d1e9bf4daac224fb230e0c9b5c56c4cab4d-dirty
  GoVersion: go1.16.6
  OsArch: linux/amd64
  Version: 3.3.1
```
Run podman info with JSON formatted response:
```
$ ./bin/podman info --format json
{
  "host": {
    "arch": "amd64",
    "buildahVersion": "1.22.3",
    "cgroupManager": "systemd",
    "cgroupVersion": "v2",
    "cgroupControllers": [],
    "conmon": {
      "package": "conmon-2.0.29-2.fc34.x86_64",
      "path": "/usr/bin/conmon",
      "version": "conmon version 2.0.29, commit: "
    },
    "cpus": 8,
    "distribution": {
      "distribution": "fedora",
      "version": "34"
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
    "kernel": "5.13.13-200.fc34.x86_64",
    "logDriver": "journald",
    "memFree": 1274040320,
    "memTotal": 16401895424,
    "ociRuntime": {
      "name": "crun",
      "package": "crun-1.0-1.fc34.x86_64",
      "path": "/usr/bin/crun",
      "version": "crun version 1.0\ncommit: 139dc6971e2f1d931af520188763e984d6cdfbf8\nspec: 1.0.0\n+SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +YAJL"
    },
    "os": "linux",
    "remoteSocket": {
      "path": "/run/user/3267/podman/podman.sock"
    },
    "serviceIsRemote": false,
    "security": {
      "apparmorEnabled": false,
      "capabilities": "CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT",
      "rootless": true,
      "seccompEnabled": true,
      "seccompProfilePath": "/usr/share/containers/seccomp.json",
      "selinuxEnabled": true
    },
    "slirp4netns": {
      "executable": "/bin/slirp4netns",
      "package": "slirp4netns-1.1.12-2.fc34.x86_64",
      "version": "slirp4netns version 1.1.12\ncommit: 7a104a101aa3278a2152351a082a6df71f57c9a3\nlibslirp: 4.4.0\nSLIRP_CONFIG_VERSION_MAX: 3\nlibseccomp: 2.5.0"
    },
    "swapFree": 16818888704,
    "swapTotal": 16886259712,
    "uptime": "33h 59m 25.69s (Approximately 1.38 days)",
    "linkmode": "dynamic"
  },
  "store": {
    "configFile": "/home/dwalsh/.config/containers/storage.conf",
    "containerStore": {
      "number": 2,
      "paused": 0,
      "running": 1,
      "stopped": 1
    },
    "graphDriverName": "overlay",
    "graphOptions": {
    },
    "graphRoot": "/home/dwalsh/.local/share/containers/storage",
    "graphStatus": {
      "Backing Filesystem": "extfs",
      "Native Overlay Diff": "true",
      "Supports d_type": "true",
      "Using metacopy": "false"
    },
    "imageStore": {
      "number": 37
    },
    "runRoot": "/run/user/3267/containers",
    "volumePath": "/home/dwalsh/.local/share/containers/storage/volumes"
  },
  "registries": {
    "localhost:5000": {
  "Prefix": "localhost:5000",
  "Location": "localhost:5000",
  "Insecure": true,
  "Mirrors": null,
  "Blocked": false,
  "MirrorByDigestOnly": false
},
    "search": [
  "registry.fedoraproject.org",
  "registry.access.redhat.com",
  "docker.io"
]
  },
  "plugins": {
    "volume": [
      "local"
    ],
    "network": [
      "bridge",
      "macvlan"
    ],
    "log": [
      "k8s-file",
      "none",
      "journald"
    ]
  },
  "version": {
    "APIVersion": "3.3.1",
    "Version": "3.3.1",
    "GoVersion": "go1.16.6",
    "GitCommit": "",
    "BuiltTime": "Mon Aug 30 16:46:36 2021",
    "Built": 1630356396,
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
