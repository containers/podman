% podman-machine-info 1

## NAME
podman\-machine\-info - Display machine host info

## SYNOPSIS
**podman machine info**

## DESCRIPTION

Display information pertaining to the machine host.
Rootless only, as all `podman machine` commands can be only be used with rootless Podman.

## OPTIONS

#### **--format**, **-f**=*format*

Change output format to "json" or a Go template.

| **Placeholder**     | **Description**                   |
| ------------------- | --------------------------------- |
| .Host ...           | Host information for local machine|
| .Version ...        | Version of the machine            |

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine info
Host:
  Arch: amd64
  CurrentMachine: ""
  DefaultMachine: ""
  EventsDir: /run/user/3267/podman
  MachineConfigDir: /home/myusername/.config/containers/podman/machine/qemu
  MachineImageDir: /home/myusername/.local/share/containers/podman/machine/qemu
  MachineState: ""
  NumberOfMachines: 0
  OS: linux
  VMType: qemu
Version:
  APIVersion: 4.4.0
  Built: 1677097848
  BuiltTime: Wed Feb 22 15:30:48 2023
  GitCommit: aa196c0d5c9abd5800edf9e27587c60343a26c2b-dirty
  GoVersion: go1.20
  Os: linux
  OsArch: linux/amd64
  Version: 4.4.0
```

```
$ podman machine info --format json
{
  "Host": {
    "Arch": "amd64",
    "CurrentMachine": "",
    "DefaultMachine": "",
    "EventsDir": "/run/user/3267/podman",
    "MachineConfigDir": "/home/myusername/.config/containers/podman/machine/qemu",
    "MachineImageDir": "/home/myusername/.local/share/containers/podman/machine/qemu",
    "MachineState": "",
    "NumberOfMachines": 0,
    "OS": "linux",
    "VMType": "qemu"
  },
  "Version": {
    "APIVersion": "4.4.0",
    "Version": "4.4.0",
    "GoVersion": "go1.20",
    "GitCommit": "aa196c0d5c9abd5800edf9e27587c60343a26c2b-dirty",
    "BuiltTime": "Wed Feb 22 15:30:48 2023",
    "Built": 1677097848,
    "OsArch": "linux/amd64",
    "Os": "linux"
  }
}
```

```
$ podman machine info --format "{{ .Host.Arch }}"
amd64

```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
June 2022, Originally compiled by Ashley Cui <acui@redhat.com>
