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
  buildahVersion: 1.23.0
  cgroupControllers: []
  cgroupManager: systemd
  cgroupVersion: v2
  conmon:
    package: conmon-2.0.29-2.fc34.x86_64
    path: /usr/bin/conmon
    version: 'conmon version 2.0.29, commit: '
 cpu_utilization:
   idle_percent: 96.84
   system_percent: 0.71
   user_percent: 2.45
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
  memFree: 1833385984
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
  swapFree: 15687475200
  swapTotal: 16886259712
  uptime: 47h 15m 9.91s (Approximately 1.96 days)
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
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - docker.io
  - quay.io
store:
  configFile: /home/dwalsh/.config/containers/storage.conf
  containerStore:
    number: 9
    paused: 0
    running: 1
    stopped: 8
  graphDriverName: overlay
  graphOptions: {}
  graphRoot: /home/dwalsh/.local/share/containers/storage
  graphRootAllocated: 510389125120
  graphRootUsed: 129170714624
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
    Using metacopy: "false"
  imageCopyTmpDir: /home/dwalsh/.local/share/containers/storage/tmp
  imageStore:
    number: 5
  runRoot: /run/user/3267/containers
  volumePath: /home/dwalsh/.local/share/containers/storage/volumes
version:
  APIVersion: 4.0.0
  Built: 1631648722
  BuiltTime: Tue Sep 14 15:45:22 2021
  GitCommit: 23677f92dd83e96d2bc8f0acb611865fb8b1a56d
  GoVersion: go1.16.6
  OsArch: linux/amd64
  Version: 4.0.0
```
Run podman info with JSON formatted response:
```
$ podman info --format json
{
  "host": {
    "arch": "amd64",
    "buildahVersion": "1.23.0",
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
    "memFree": 1785753600,
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
    "swapFree": 15687475200,
    "swapTotal": 16886259712,
    "uptime": "47h 17m 29.75s (Approximately 1.96 days)",
    "linkmode": "dynamic"
  },
  "store": {
    "configFile": "/home/dwalsh/.config/containers/storage.conf",
    "containerStore": {
      "number": 9,
      "paused": 0,
      "running": 1,
      "stopped": 8
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
    "imageCopyTmpDir": "/home/dwalsh/.local/share/containers/storage/tmp",
    "imageStore": {
      "number": 5
    },
    "runRoot": "/run/user/3267/containers",
    "volumePath": "/home/dwalsh/.local/share/containers/storage/volumes"
  },
  "registries": {
    "search": [
  "registry.fedoraproject.org",
  "registry.access.redhat.com",
  "docker.io",
  "quay.io"
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
    "APIVersion": "4.0.0",
    "Version": "4.0.0",
    "GoVersion": "go1.16.6",
    "GitCommit": "23677f92dd83e96d2bc8f0acb611865fb8b1a56d",
    "BuiltTime": "Tue Sep 14 15:45:22 2021",
    "Built": 1631648722,
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
**[podman(1)](podman.1.md)**, **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**, **[containers-storage.conf(5)](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md)**
