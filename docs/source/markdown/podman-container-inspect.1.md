% podman-container-inspect(1)

## NAME
podman\-container\-inspect - Display a container's configuration

## SYNOPSIS
**podman container inspect** [*options*] *container* [*container* ...]

## DESCRIPTION

This displays the low-level information on containers identified by name or ID. By default, this will render
all results in a JSON array. If a format is specified, the given template will be executed for each result.

## OPTIONS

#### **--format**, **-f**=*format*

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

(This option is not available with the remote Podman client.)

#### **--size**, **-s**

In addition to normal output, display the total file size if the type is a container.


## EXAMPLE

```
$ podman container inspect foobar
[
    {
        "Id": "99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6",
        "Created": "2021-09-16T06:09:08.936623325-04:00",
        "Path": "echo",
        "Args": [
            "hi"
        ],
        "State": {
            "OciVersion": "1.0.2-dev",
            "Status": "exited",
            "Running": false,
            "Paused": false,
            "Restarting": false,
            "OOMKilled": false,
            "Dead": false,
            "Pid": 0,
            "ExitCode": 0,
            "Error": "",
            "StartedAt": "2021-09-16T06:09:09.033564436-04:00",
            "FinishedAt": "2021-09-16T06:09:09.036184314-04:00",
            "Healthcheck": {
                "Status": "",
                "FailingStreak": 0,
                "Log": null
            }
        },
        "Image": "14119a10abf4669e8cdbdff324a9f9605d99697215a0d21c360fe8dfa8471bab",
        "ImageName": "docker.io/library/alpine:latest",
        "Rootfs": "",
        "Pod": "",
        "ResolvConfPath": "/run/user/3267/containers/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/resolv.conf",
        "HostnamePath": "/run/user/3267/containers/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/hostname",
        "HostsPath": "/run/user/3267/containers/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/hosts",
        "StaticDir": "/home/dwalsh/.local/share/containers/storage/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata",
        "OCIConfigPath": "/home/dwalsh/.local/share/containers/storage/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/config.json",
        "OCIRuntime": "crun",
        "ConmonPidFile": "/run/user/3267/containers/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/conmon.pid",
        "PidFile": "/run/user/3267/containers/overlay-containers/99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6/userdata/pidfile",
        "Name": "foobar",
        "RestartCount": 0,
        "Driver": "overlay",
        "MountLabel": "system_u:object_r:container_file_t:s0:c25,c695",
        "ProcessLabel": "system_u:system_r:container_t:s0:c25,c695",
        "AppArmorProfile": "",
        "EffectiveCaps": [
            "CAP_CHOWN",
            "CAP_DAC_OVERRIDE",
            "CAP_FOWNER",
            "CAP_FSETID",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE",
            "CAP_SETFCAP",
            "CAP_SETGID",
            "CAP_SETPCAP",
            "CAP_SETUID",
            "CAP_SYS_CHROOT"
        ],
        "BoundingCaps": [
            "CAP_CHOWN",
            "CAP_DAC_OVERRIDE",
            "CAP_FOWNER",
            "CAP_FSETID",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE",
            "CAP_SETFCAP",
            "CAP_SETGID",
            "CAP_SETPCAP",
            "CAP_SETUID",
            "CAP_SYS_CHROOT"
        ],
        "ExecIDs": [],
        "GraphDriver": {
            "Name": "overlay",
            "Data": {
                "LowerDir": "/home/dwalsh/.local/share/containers/storage/overlay/e2eb06d8af8218cfec8210147357a68b7e13f7c485b991c288c2d01dc228bb68/diff",
                "UpperDir": "/home/dwalsh/.local/share/containers/storage/overlay/8f3d70434a3db17410ec4710caf4f251f3e4ed0a96a08124e4b3d4af0a0ea300/diff",
                "WorkDir": "/home/dwalsh/.local/share/containers/storage/overlay/8f3d70434a3db17410ec4710caf4f251f3e4ed0a96a08124e4b3d4af0a0ea300/work"
            }
        },
        "Mounts": [],
        "Dependencies": [],
        "NetworkSettings": {
            "EndpointID": "",
            "Gateway": "",
            "IPAddress": "",
            "IPPrefixLen": 0,
            "IPv6Gateway": "",
            "GlobalIPv6Address": "",
            "GlobalIPv6PrefixLen": 0,
            "MacAddress": "",
            "Bridge": "",
            "SandboxID": "",
            "HairpinMode": false,
            "LinkLocalIPv6Address": "",
            "LinkLocalIPv6PrefixLen": 0,
            "Ports": {},
            "SandboxKey": ""
        },
        "ExitCommand": [
            "/usr/bin/podman",
            "--root",
            "/home/dwalsh/.local/share/containers/storage",
            "--runroot",
            "/run/user/3267/containers",
            "--log-level",
            "warning",
            "--cgroup-manager",
            "systemd",
            "--tmpdir",
            "/run/user/3267/libpod/tmp",
            "--runtime",
            "crun",
            "--storage-driver",
            "overlay",
            "--events-backend",
            "journald",
            "container",
            "cleanup",
            "99f66530fe9c7249f7cf29f78e8661669d5831cbe4ee80ea757d5e922dd6a8a6"
        ],
        "Namespace": "",
        "IsInfra": false,
        "Config": {
            "Hostname": "99f66530fe9c",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                "TERM=xterm",
                "container=podman",
                "HOME=/root",
                "HOSTNAME=99f66530fe9c"
            ],
            "Cmd": [
                "echo",
                "hi"
            ],
            "Image": "docker.io/library/alpine:latest",
            "Volumes": null,
            "WorkingDir": "/",
            "Entrypoint": "",
            "OnBuild": null,
            "Labels": null,
            "Annotations": {
                "io.container.manager": "libpod",
                "io.kubernetes.cri-o.Created": "2021-09-16T06:09:08.936623325-04:00",
                "io.kubernetes.cri-o.TTY": "false",
                "io.podman.annotations.autoremove": "FALSE",
                "io.podman.annotations.init": "FALSE",
                "io.podman.annotations.privileged": "FALSE",
                "io.podman.annotations.publish-all": "FALSE",
                "org.opencontainers.image.stopSignal": "15"
            },
            "StopSignal": 15,
            "CreateCommand": [
                "podman",
                "run",
                "--name",
                "foobar",
                "alpine",
                "echo",
                "hi"
            ],
            "Timezone": "local",
            "Umask": "0022",
            "Timeout": 0,
            "StopTimeout": 10
        },
        "HostConfig": {
            "Binds": [],
            "CgroupManager": "systemd",
            "CgroupMode": "private",
            "ContainerIDFile": "",
            "LogConfig": {
                "Type": "journald",
                "Config": null,
                "Path": "",
                "Tag": "",
                "Size": "0B"
            },
            "NetworkMode": "slirp4netns",
            "PortBindings": {},
            "RestartPolicy": {
                "Name": "",
                "MaximumRetryCount": 0
            },
            "AutoRemove": false,
            "VolumeDriver": "",
            "VolumesFrom": null,
            "CapAdd": [],
            "CapDrop": [
                "CAP_AUDIT_WRITE",
                "CAP_MKNOD",
                "CAP_NET_RAW"
            ],
            "Dns": [],
            "DnsOptions": [],
            "DnsSearch": [],
            "ExtraHosts": [],
            "GroupAdd": [],
            "IpcMode": "private",
            "Cgroup": "",
            "Cgroups": "default",
            "Links": null,
            "OomScoreAdj": 0,
            "PidMode": "private",
            "Privileged": false,
            "PublishAllPorts": false,
            "ReadonlyRootfs": false,
            "SecurityOpt": [],
            "Tmpfs": {},
            "UTSMode": "private",
            "UsernsMode": "",
            "ShmSize": 65536000,
            "Runtime": "oci",
            "ConsoleSize": [
                0,
                0
            ],
            "Isolation": "",
            "CpuShares": 0,
            "Memory": 0,
            "NanoCpus": 0,
            "CgroupParent": "user.slice",
            "BlkioWeight": 0,
            "BlkioWeightDevice": null,
            "BlkioDeviceReadBps": null,
            "BlkioDeviceWriteBps": null,
            "BlkioDeviceReadIOps": null,
            "BlkioDeviceWriteIOps": null,
            "CpuPeriod": 0,
            "CpuQuota": 0,
            "CpuRealtimePeriod": 0,
            "CpuRealtimeRuntime": 0,
            "CpusetCpus": "",
            "CpusetMems": "",
            "Devices": [],
            "DiskQuota": 0,
            "KernelMemory": 0,
            "MemoryReservation": 0,
            "MemorySwap": 0,
            "MemorySwappiness": 0,
            "OomKillDisable": false,
            "PidsLimit": 2048,
            "Ulimits": [],
            "CpuCount": 0,
            "CpuPercent": 0,
            "IOMaximumIOps": 0,
            "IOMaximumBandwidth": 0,
            "CgroupConf": null
        }
    }
]
```

```
$ podman container inspect nervous_fermi --format "{{.ImageName}}"
registry.access.redhat.com/ubi8:latest
```

```
$ podman container inspect foobar --format "{{.GraphDriver.Name}}"
overlay
```

```
$ podman container inspect --latest --format {{.EffectiveCaps}}
[CAP_CHOWN CAP_DAC_OVERRIDE CAP_FOWNER CAP_FSETID CAP_KILL CAP_NET_BIND_SERVICE CAP_SETFCAP CAP_SETGID CAP_SETPCAP CAP_SETUID CAP_SYS_CHROOT]
```

## SEE ALSO
**[podman(1)](podman.1.md)**,**[podman-container(1)](podman-container.1.md)**, **[podman-inspect(1)](podman-inspect.1.md)**

## HISTORY
Sep 2021, Originally compiled by Dan Walsh <dwalsh@redhat.com>
