% podman-systemd.unit 5

## NAME

podman\-systemd.unit - systemd units using Podman Quadlet

## SYNOPSIS

*name*.container, *name*.volume, *name*.network, `*.kube`

### Podman unit search path

 * /etc/containers/systemd/
 * /usr/share/containers/systemd/

### Podman user unit search path

 * $XDG_CONFIG_HOME/containers/systemd/ or ~/.config/containers/systemd/
 * /etc/containers/systemd/users/$(UID)
 * /etc/containers/systemd/users/

## DESCRIPTION

Podman supports starting containers (and creating volumes) via systemd by using a
[systemd generator](https://www.freedesktop.org/software/systemd/man/systemd.generator.html).
These files are read during boot (and when `systemctl daemon-reload` is run) and generate
corresponding regular systemd service unit files. Both system and user systemd units are supported.

The Podman generator reads the search paths above and reads files with the extensions `.container`
`.volume` and `*.kube`, and for each file generates a similarly named `.service` file. Be aware that
existing vendor services (i.e., in `/usr/`) are replaced if they have the same name. The generated unit files can
be started and managed with `systemctl` like any other systemd service. `systemctl {--user} list-unit-files`
lists existing unit files on the system.

Files with the `.network` extension are only read if they are mentioned in a `.container` file. See the `Network=` key.

The Podman files use the same format as [regular systemd unit files](https://www.freedesktop.org/software/systemd/man/systemd.syntax.html).
Each file type has a custom section (for example, `[Container]`) that is handled by Podman, and all
other sections are passed on untouched, allowing the use of any normal systemd configuration options
like dependencies or cgroup limits.

For rootless containers, when administrators place Quadlet files in the
/etc/containers/systemd/users directory, all users' sessions execute the
Quadlet when the login session begins. If the administrator places a Quadlet
file in the /etc/containers/systemd/users/${UID}/ directory, then only the
user with the matching UID execute the Quadlet when the login
session gets started. For unit files placed in subdirectories within
/etc/containers/systemd/user/${UID}/ and the other user unit search paths,
Quadlet will recursively search and run the unit files present in these subdirectories.


### Enabling unit files

The services created by Podman are considered transient by systemd, which means they don't have the same
persistence rules as regular units. In particular, it is not possible to "systemctl enable" them
in order for them to become automatically enabled on the next boot.

To compensate for this, the generator manually applies the `[Install]` section of the container definition
unit files during generation, in the same way `systemctl enable` does when run later.

For example, to start a container on boot, add something like this to the file:

```
[Install]
WantedBy=default.target
```

Currently, only the `Alias`, `WantedBy` and `RequiredBy` keys are supported.

**NOTE:** To express dependencies between containers, use the generated names of the service. In other
words `WantedBy=other.service`, not `WantedBy=other.container`. The same is
true for other kinds of dependencies, too, like `After=other.service`.

### Debugging unit files

After placing the unit file in one of the unit search paths (mentioned above), you can start it with
`systemctl start {--user}`. If it fails with "Failed to start example.service: Unit example.service not found.",
then it is possible that you used incorrect syntax or you used an option from a newer version of Podman
Quadlet and the generator failed to create a service file.

View the generated files and/or error messages with:
```
/usr/lib/systemd/system-generators/podman-system-generator {--user} --dryrun
```

#### Debugging a limited set of unit files

If you would like to debug a limited set of unit files, you can copy them to a separate directory and set the
`QUADLET_UNIT_DIRS` environment variable to this directory when running the command below:

```
QUADLET_UNIT_DIRS=<Directory> /usr/lib/systemd/system-generators/podman-system-generator {--user} --dryrun
```

This will instruct quadlet to look for units in this directory instead of the common ones and by
that limit the output to only the units you are debugging.

## Container units [Container]

Container units are named with a `.container` extension and contain a `[Container]` section describing
the container that is run as a service. The resulting service file contains a line like
`ExecStart=podman run … image-name`, and most of the keys in this section control the command-line
options passed to Podman. However, some options also affect the details of how systemd is set up to run and
interact with the container.

By default, the Podman container has the same name as the unit, but with a `systemd-` prefix, i.e.
a `$name.container` file creates a `$name.service` unit and a `systemd-$name` Podman container. The
`ContainerName` option allows for overriding this default name with a user-provided one.

There is only one required key, `Image`, which defines the container image the service runs.

Valid options for `[Container]` are listed below:

| **[Container] options**        | **podman run equivalent**                            |
|--------------------------------|------------------------------------------------------|
| AddCapability=CAP              | --cap-add CAP                                        |
| AddDevice=/dev/foo             | --device /dev/foo                                    |
| Annotation="YXZ"               | --annotation "XYZ"                                   |
| AutoUpdate=registry            | --label "io.containers.autoupdate=registry"          |
| ContainerName=name             | --name name                                          |
| DropCapability=CAP             | --cap-drop=CAP                                       |
| Environment=foo=bar            | --env foo=bar                                        |
| EnvironmentFile=/tmp/env       | --env-file /tmp/env                                  |
| EnvironmentHost=true           | --env-host                                           |
| Exec=/usr/bin/command          | Command after image specification - /usr/bin/command |
| ExposeHostPort=50-59           | --expose 50-59                                       |
| Group=1234                     | --user UID:1234                                      |
| HealthCmd="/usr/bin/command"   | --health-cmd="/usr/bin/command"                      |
| HealthInterval=2m              | --health-interval=2m                                 |
| HealthOnFailure=kill           | --health-on-failure=kill                             |
| HealthRetries=5                | --health-retries=5                                   |
| HealthStartPeriod=1m           | --health-start-period=period=1m                      |
| HealthStartupCmd="command"     | --health-startup-cmd="command"                       |
| HealthStartupInterval=1m       | --health-startup-interval=2m                         |
| HealthStartupRetries=8         | --health-startup-retries=8                           |
| HealthStartupSuccess=2         | --health-startup-success=2                           |
| HealthStartupTimeout=1m33s     | --health-startup-timeout=1m33s                       |
| HealthTimeout=20s              | --health-timeout=20s                                 |
| HostName=new-host-name         | --hostname="new-host-name"                           |
| Image=ubi8                     | Image specification - ubi8                           |
| IP=192.5.0.1                   | --ip 192.5.0.0                                       |
| IP6=fd46:db93:aa76:ac37::10    | --ip6 2001:db8::1                                    |
| Label="YXZ"                    | --label "XYZ"                                        |
| LogDriver=journald             | --log-driver journald                                |
| Mount=type=...                 | --mount type=...                                     |
| Network=host                   | --net host                                           |
| NoNewPrivileges=true           | --security-opt no-new-privileges                     |
| Rootfs=/var/lib/rootfs         | --rootfs /var/lib/rootfs                             |
| Notify=true                    | --sdnotify container                                 |
| PodmanArgs=--add-host foobar   | --add-host foobar                                    |
| PublishPort=50-59              | --publish 50-59                                      |
| Pull=never                     | --pull=never                                         |
| ReadOnly=true                  | --read-only                                          |
| RunInit=true                   | --init                                               |
| SeccompProfile=/tmp/s.json     | --security-opt seccomp=/tmp/s.json                   |
| SecurityLabelDisable=true      | --security-opt label=disable                         |
| SecurityLabelFileType=usr_t    | --security-opt label=filetype:usr_t                  |
| SecurityLabelLevel=s0:c1,c2    | --security-opt label=level:s0:c1,c2                  |
| SecurityLabelNested=true       | --security-opt label=nested                          |
| SecurityLabelType=spc_t        | --security-opt label=type:spc_t                      |
| Sysctl=name=value              | --sysctl=name=value                                  |
| Timezone=local                 | --tz local                                           |
| Tmpfs=/work                    | --tmpfs /work                                        |
| User=bin                       | --user bin                                           |
| UserNS=keep-id:uid=200,gid=210 | --userns keep-id:uid=200,gid=210                     |
| VolatileTmp=true               | --tmpfs /tmp                                         |
| Volume=/source:/dest           | --volume /source:/dest                               |
| WorkingDir=$HOME               | --workdir $HOME                                      |

Description of `[Container]` section are:

### `AddCapability=`

Add these capabilities, in addition to the default Podman capability set, to the container.

This is a space separated list of capabilities. This key can be listed multiple times.

For example:
```
AddCapability=CAP_DAC_OVERRIDE CAP_IPC_OWNER
```

### `AddDevice=`

Adds a device node from the host into the container. The format of this is
`HOST-DEVICE[:CONTAINER-DEVICE][:PERMISSIONS]`, where `HOST-DEVICE` is the path of
the device node on the host, `CONTAINER-DEVICE` is the path of the device node in
the container, and `PERMISSIONS` is a list of permissions combining 'r' for read,
'w' for write, and 'm' for mknod(2). The `-` prefix tells Quadlet to add the device
only if it exists on the host.

This key can be listed multiple times.

### `Annotation=`

Set one or more OCI annotations on the container. The format is a list of `key=value` items,
similar to `Environment`.

This key can be listed multiple times.

### `AutoUpdate=`

Indicates whether the container will be auto-updated ([podman-auto-update(1)](podman-auto-update.1.md)). The following values are supported:

* `registry`: Requires a fully-qualified image reference (e.g., quay.io/podman/stable:latest) to be used to create the container. This enforcement is necessary to know which image to actually check and pull. If an image ID was used, Podman does not know which image to check/pull anymore.

* `local`: Tells Podman to compare the image a container is using to the image with its raw name in local storage. If an image is updated locally, Podman simply restarts the systemd unit executing the container.

### `ContainerName=`

The (optional) name of the Podman container. If this is not specified, the default value
of `systemd-%N` is used, which is the same as the service name but with a `systemd-`
prefix to avoid conflicts with user-managed containers.

### `DropCapability=`

Drop these capabilities from the default podman capability set, or `all` to drop all capabilities.

This is a space separated list of capabilities. This key can be listed multiple times.

For example:
```
DropCapability=CAP_DAC_OVERRIDE CAP_IPC_OWNER
```

### `Environment=`

Set an environment variable in the container. This uses the same format as
[services in systemd](https://www.freedesktop.org/software/systemd/man/systemd.exec.html#Environment=)
and can be listed multiple times.

### `EnvironmentFile=`

Use a line-delimited file to set environment variables in the container.
The path may be absolute or relative to the location of the unit file.
This key may be used multiple times, and the order persists when passed to `podman run`.

### `EnvironmentHost=`

Use the host environment inside of the container.

### `Exec=`

If this is set then it defines what command line to run in the container. If it is not set the
default entry point of the container image is used. The format is the same as for
[systemd command lines](https://www.freedesktop.org/software/systemd/man/systemd.service.html#Command%20lines).

### `ExposeHostPort=`

Exposes a port, or a range of ports (e.g. `50-59`), from the host to the container. Equivalent
to the Podman `--expose` option.

This key can be listed multiple times.

### `Group=`

The (numeric) GID to run as inside the container. This does not need to match the GID on the host,
which can be modified with `UsersNS`, but if that is not specified, this GID is also used on the host.

### `HealthCmd=`

Set or alter a healthcheck command for a container. A value of none disables existing healthchecks.
Equivalent to the Podman `--health-cmd` option.

### `HealthInterval=`

Set an interval for the healthchecks. An interval of disable results in no automatic timer setup.
Equivalent to the Podman `--health-interval` option.

### `HealthOnFailure=`

Action to take once the container transitions to an unhealthy state.
The "kill" action in combination integrates best with systemd. Once
the container turns unhealthy, it gets killed, and systemd restarts the
service.
Equivalent to the Podman `--health-on-failure` option.

### `HealthRetries=`

The number of retries allowed before a healthcheck is considered to be unhealthy.
Equivalent to the Podman `--health-retries` option.

### `HealthStartPeriod=`

The initialization time needed for a container to bootstrap.
Equivalent to the Podman `--health-start-period` option.

### `HealthStartupCmd=`

Set a startup healthcheck command for a container.
Equivalent to the Podman `--health-startup-cmd` option.

### `HealthStartupInterval=`

Set an interval for the startup healthcheck. An interval of disable results in no automatic timer setup.
Equivalent to the Podman `--health-startup-interval` option.

### `HealthStartupRetries=`

The number of attempts allowed before the startup healthcheck restarts the container.
Equivalent to the Podman `--health-startup-retries` option.

### `HealthStartupSuccess=`

The number of successful runs required before the startup healthcheck succeeds and the regular healthcheck begins.
Equivalent to the Podman `--health-startup-success` option.

### `HealthStartupTimeout=`

The maximum time a startup healthcheck command has to complete before it is marked as failed.
Equivalent to the Podman `--health-startup-timeout` option.

### `HealthTimeout=`

The maximum time allowed to complete the healthcheck before an interval is considered failed.
Equivalent to the Podman `--health-timeout` option.

### `HostName=`

Sets the host name that is available inside the container.
Equivalent to the Podman `--hostname` option.

### `Image=`

The image to run in the container.
It is recommended to use a fully qualified image name rather than a short name, both for
performance and robustness reasons.

The format of the name is the same as when passed to `podman run`, so it supports e.g., using
`:tag` or using digests guarantee a specific image version.

### `IP=`

Specify a static IPv4 address for the container, for example **10.88.64.128**.
Equivalent to the Podman `--ip` option.

### `IP6=`

Specify a static IPv6 address for the container, for example **fd46:db93:aa76:ac37::10**.
Equivalent to the Podman `--ip6` option.

### `Label=`

Set one or more OCI labels on the container. The format is a list of `key=value` items,
similar to `Environment`.

This key can be listed multiple times.

### `LogDriver=`

Set the log-driver used by Podman when running the container.
Equivalent to the Podman `--log-driver` option.

### `Mount=`

Attach a filesystem mount to the container.
This is equivalent to the Podman `--mount` option, and
generally has the form `type=TYPE,TYPE-SPECIFIC-OPTION[,...]`.

As a special case, for `type=volume` if `source` ends with `.volume`, a Podman named volume called
`systemd-$name` is used as the source, and the generated systemd service contains
a dependency on the `$name-volume.service`. Such a volume can be automatically be lazily
created by using a `$name.volume` Quadlet file.

This key can be listed multiple times.

### `Network=`

Specify a custom network for the container. This has the same format as the `--network` option
to `podman run`. For example, use `host` to use the host network in the container, or `none` to
not set up networking in the container.

As a special case, if the `name` of the network ends with `.network`, a Podman network called
`systemd-$name` is used, and the generated systemd service contains
a dependency on the `$name-network.service`. Such a network can be automatically
created by using a `$name.network` Quadlet file.

This key can be listed multiple times.

### `NoNewPrivileges=` (defaults to `no`)

If enabled, this disables the container processes from gaining additional privileges via things like
setuid and file capabilities.

### `Rootfs=`

The rootfs to use for the container. Rootfs points to a directory on the system that contains the content to be run within the container. This option conflicts with the `Image` option.

The format of the rootfs is the same as when passed to `podman run --rootfs`, so it supports ovelay mounts as well.

Note: On SELinux systems, the rootfs needs the correct label, which is by default unconfined_u:object_r:container_file_t:s0.

### `Notify=` (defaults to `no`)

By default, Podman is run in such a way that the systemd startup notify command is handled by
the container runtime. In other words, the service is deemed started when the container runtime
starts the child in the container. However, if the container application supports
[sd_notify](https://www.freedesktop.org/software/systemd/man/sd_notify.html), then setting
`Notify` to true passes the notification details to the container allowing it to notify
of startup on its own.

### `PodmanArgs=`

This key contains a list of arguments passed directly to the end of the `podman run` command
in the generated file (right before the image name in the command line). It can be used to
access Podman features otherwise unsupported by the generator. Since the generator is unaware
of what unexpected interactions can be caused by these arguments, is not recommended to use
this option.

The format of this is a space separated list of arguments, which can optionally be individually
escaped to allow inclusion of whitespace and other control characters.

This key can be listed multiple times.

### `PublishPort=`

Exposes a port, or a range of ports (e.g. `50-59`), from the container to the host. Equivalent
to the Podman `--publish` option. The format is similar to the Podman options, which is of
the form `ip:hostPort:containerPort`, `ip::containerPort`, `hostPort:containerPort` or
`containerPort`, where the number of host and container ports must be the same (in the case
of a range).

If the IP is set to 0.0.0.0 or not set at all, the port is bound on all IPv4 addresses on
the host; use [::] for IPv6.

Note that not listing a host port means that Podman automatically selects one, and it
may be different for each invocation of service. This makes that a less useful option. The
allocated port can be found with the `podman port` command.

This key can be listed multiple times.

### `Pull=`

Set the image pull policy.
This is equivalent to the Podman `--pull` option

### `ReadOnly=` (defaults to `no`)

If enabled, makes the image read-only.

### `RunInit=` (default to `no`)

If enabled, the container has a minimal init process inside the
container that forwards signals and reaps processes.

### `SeccompProfile=`

Set the seccomp profile to use in the container. If unset, the default podman profile is used.
Set to either the pathname of a json file, or `unconfined` to disable the seccomp filters.

### `SecurityLabelDisable=`

Turn off label separation for the container.

### `SecurityLabelFileType=`

Set the label file type for the container files.

### `SecurityLabelLevel=`

Set the label process level for the container processes.

### `SecurityLabelNested=`

Allow SecurityLabels to function within the container. This allows separation of containers created within the container.

### `SecurityLabelType=`

Set the label process type for the container processes.

### `Secret=`

Use a Podman secret in the container either as a file or an environment variable.
This is equivalent to the Podman `--secret` option and generally has the form `secret[,opt=opt ...]`

### `Sysctl=`

Configures namespaced kernel parameters for the container. The format is `Sysctl=name=value`.

This is a space separated list of kernel parameters. This key can be listed multiple times.

For example:
```
Sysctl=net.ipv6.conf.all.disable_ipv6=1 net.ipv6.conf.all.use_tempaddr=1
```

### `Tmpfs=`

Mount a tmpfs in the container. This is equivalent to the Podman `--tmpfs` option, and
generally has the form `CONTAINER-DIR[:OPTIONS]`.

This key can be listed multiple times.

### `Timezone=` (if unset uses system-configured default)

The timezone to run the container in.

### `User=`

The (numeric) UID to run as inside the container. This does not need to match the UID on the host,
which can be modified with `UserNS`, but if that is not specified, this UID is also used on the host.

### `UserNS=`

Set the user namespace mode for the container. This is equivalent to the Podman `--userns` option and
generally has the form `MODE[:OPTIONS,...]`.

### `VolatileTmp=` (defaults to `no`)

If enabled, the container has a fresh tmpfs mounted on `/tmp`.

**NOTE:** Podman automatically copies any content from the image onto the tmpfs

### `Volume=`

Mount a volume in the container. This is equivalent to the Podman `--volume` option, and
generally has the form `[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]`.

If `SOURCE-VOLUME` starts with `.`, Quadlet resolves the path relative to the location of the unit file.

As a special case, if `SOURCE-VOLUME` ends with `.volume`, a Podman named volume called
`systemd-$name` is used as the source, and the generated systemd service contains
a dependency on the `$name-volume.service`. Such a volume can be automatically be lazily
created by using a `$name.volume` Quadlet file.

This key can be listed multiple times.

### `WorkingDir=`

Working directory inside the container.

The default working directory for running binaries within a container is the root directory (/). The image developer can set a different default with the WORKDIR instruction. This option overrides the working directory by using the -w option.

## Kube units [Kube]

Kube units are named with a `.kube` extension and contain a `[Kube]` section describing
how `podman kube play` runs as a service. The resulting service file contains a line like
`ExecStart=podman kube play … file.yml`, and most of the keys in this section control the command-line
options passed to Podman. However, some options also affect the details of how systemd is set up to run and
interact with the container.

There is only one required key, `Yaml`, which defines the path to the Kubernetes YAML file.

Valid options for `[Kube]` are listed below:

| **[Kube] options**                  | **podman kube play equivalent**                                  |
| ----------------------------------- | -----------------------------------------------------------------|
| AutoUpdate=registry                 | --annotation "io.containers.autoupdate=registry"                 |
| ConfigMap=/tmp/config.map           | --config-map /tmp/config.map                                     |
| LogDriver=journald                  | --log-driver journald                                            |
| Network=host                        | --net host                                                       |
| PodmanArgs=\-\-annotation=key=value | --annotation=key=value                                           |
| PublishPort=59-60                   | --publish=59-60                                                  |
| SetWorkingDirectory=yaml            | Set `WorkingDirectory` of unit file to location of the YAML file |
| UserNS=keep-id:uid=200,gid=210      | --userns keep-id:uid=200,gid=210                                 |
| Yaml=/tmp/kube.yaml                 | podman kube play /tmp/kube.yaml                                  |

Supported keys in the `[Kube]` section are:

### `AutoUpdate=`

Indicates whether containers will be auto-updated ([podman-auto-update(1)](podman-auto-update.1.md)). AutoUpdate can be specified multiple times. The following values are supported:

* `registry`: Requires a fully-qualified image reference (e.g., quay.io/podman/stable:latest) to be used to create the container. This enforcement is necessary to know which images to actually check and pull. If an image ID was used, Podman does not know which image to check/pull anymore.

* `local`: Tells Podman to compare the image a container is using to the image with its raw name in local storage. If an image is updated locally, Podman simply restarts the systemd unit executing the Kubernetes quadlet.

* `name/(local|registry)`: Tells Podman to perform the `local` or `registry` autoupdate on the specified container name.

### `ConfigMap=`

Pass the Kubernetes ConfigMap YAML at path to `podman kube play` via the `--configmap` argument.
Unlike the `configmap` argument, the value may contain only one path but
it may be absolute or relative to the location of the unit file.

This key may be used multiple times

### `ExitCodePropagation=`

Control how the main PID of the systemd service should exit.  The following values are supported:
- `all`: exit non-zero if all containers have failed (i.e., exited non-zero)
- `any`: exit non-zero if any container has failed
- `none`: exit zero and ignore failed containers

The current default value is `none`.

### `LogDriver=`

Set the log-driver Podman uses when running the container.
Equivalent to the Podman `--log-driver` option.

### `Mask=`

Specify the paths to mask separated by a colon. `Mask=/path/1:/path/2`.  A masked path cannot be accessed inside the container.

### `Network=`

Specify a custom network for the container. This has the same format as the `--network` option
to `podman kube play`. For example, use `host` to use the host network in the container, or `none` to
not set up networking in the container.

As a special case, if the `name` of the network ends with `.network`, a Podman network called
`systemd-$name` is used, and the generated systemd service contains
a dependency on the `$name-network.service`. Such a network can be automatically
created by using a `$name.network` Quadlet file.

This key can be listed multiple times.

### `PodmanArgs=`

This key contains a list of arguments passed directly to the end of the `podman kube play` command
in the generated file (right before the path to the yaml file in the command line). It can be used to
access Podman features otherwise unsupported by the generator. Since the generator is unaware
of what unexpected interactions can be caused by these arguments, is not recommended to use
this option.

The format of this is a space separated list of arguments, which can optionally be individually
escaped to allow inclusion of whitespace and other control characters.

This key can be listed multiple times.

### `PublishPort=`

Exposes a port, or a range of ports (e.g. `50-59`), from the container to the host. Equivalent
to the `podman kube play`'s `--publish` option. The format is similar to the Podman options, which is of
the form `ip:hostPort:containerPort`, `ip::containerPort`, `hostPort:containerPort` or
`containerPort`, where the number of host and container ports must be the same (in the case
of a range).

If the IP is set to 0.0.0.0 or not set at all, the port is bound on all IPv4 addresses on
the host; use [::] for IPv6.

The list of published ports specified in the unit file is merged with the list of ports specified
in the Kubernetes YAML file. If the same container port and protocol is specified in both, the
entry from the unit file takes precedence

This key can be listed multiple times.

### `SetWorkingDirectory=`

Set the `WorkingDirectory` field of the `Service` group of the Systemd service unit file.
Used to allow `podman kube play` to correctly resolve relative paths.
Supported values are `yaml` and `unit` to set the working directory to that of the YAML or Quadlet Unit file respectively.

Alternatively, users can explicitly set the `WorkingDirectory` field of the `Service` group in the `.kube` file.
Please note that if the `WorkingDirectory` field of the `Service` group is set,
Quadlet will not set it even if `SetWorkingDirectory` is set

### `Unmask=`

Specify the paths to unmask separated by a colon. unmask=ALL or /path/1:/path/2, or shell expanded paths (/proc/*):

If set to `ALL`, Podman will unmask all the paths that are masked or made read-only by default.

The default masked paths are /proc/acpi, /proc/kcore, /proc/keys, /proc/latency_stats, /proc/sched_debug, /proc/scsi, /proc/timer_list, /proc/timer_stats, /sys/firmware, and /sys/fs/selinux.

The default paths that are read-only are /proc/asound, /proc/bus, /proc/fs, /proc/irq, /proc/sys, /proc/sysrq-trigger, /sys/fs/cgroup.

### `UserNS=`

Set the user namespace mode for the container. This is equivalent to the Podman `--userns` option and
generally has the form `MODE[:OPTIONS,...]`.

### `Yaml=`

The path, absolute or relative to the location of the unit file, to the Kubernetes YAML file to use.

## Network units [Network]

Network files are named with a `.network` extension and contain a section `[Network]` describing the
named Podman network. The generated service is a one-time command that ensures that the network
exists on the host, creating it if needed.

By default, the Podman network has the same name as the unit, but with a `systemd-` prefix, i.e. for
a network file named `$NAME.network`, the generated Podman network is called `systemd-$NAME`, and
the generated service file is `$NAME-network.service`. The `NetworkName` option allows for
overriding this default name with a user-provided one.

Using network units allows containers to depend on networks being automatically pre-created. This is
particularly interesting when using special options to control network creation, as Podman otherwise creates networks with the default options.

Valid options for `[Network]` are listed below:

| **[Network] options**         | **podman network create equivalent** |
|-------------------------------|--------------------------------------|
| DisableDNS=true               | --disable-dns                        |
| Driver=bridge                 | --driver bridge                      |
| Gateway=192.168.55.3          | --gateway 192.168.55.3               |
| Internal=true                 | --internal                           |
| IPAMDriver=dhcp               | --ipam-driver dhcp                   |
| IPRange=192.168.55.128/25     | --ip-range 192.168.55.128/25         |
| IPv6=true                     | --ipv6                               |
| Label="YXZ"                   | --label "XYZ"                        |
| NetworkName=foo               | podman network create foo            |
| Options=isolate               | --opt isolate                        |
| PodmanArgs=--dns=192.168.55.1 | --dns=192.168.55.1                   |
| Subnet=192.5.0.0/16           | --subnet 192.5.0.0/16                |

Supported keys in `[Network]` section are:

### `DisableDNS=` (defaults to `no`)

If enabled, disables the DNS plugin for this network.

This is equivalent to the Podman `--disable-dns` option

### `Driver=` (defaults to `bridge`)

Driver to manage the network. Currently `bridge`, `macvlan` and `ipvlan` are supported.

This is equivalent to the Podman `--driver` option

### `Gateway=`

Define a gateway for the subnet. If you want to provide a gateway address, you must also provide a subnet option.

This is equivalent to the Podman `--gateway` option

This key can be listed multiple times.

### `Internal=` (defaults to `no`)

Restrict external access of this network.

This is equivalent to the Podman `--internal` option

### `IPAMDriver=`

Set the ipam driver (IP Address Management Driver) for the network. Currently `host-local`, `dhcp` and `none` are supported.

This is equivalent to the Podman `--ipam-driver` option

### `IPRange=`

Allocate  container  IP  from a range. The range must be a either a complete subnet in CIDR notation or be
in the `<startIP>-<endIP>` syntax which allows for a more flexible range compared to the CIDR subnet.
The ip-range option must be used with a subnet option.

This is equivalent to the Podman `--ip-range` option

This key can be listed multiple times.

### `IPv6=`

Enable IPv6 (Dual Stack) networking.

This is equivalent to the Podman `--ipv6` option

### `Label=`

Set one or more OCI labels on the network. The format is a list of
`key=value` items, similar to `Environment`.

This key can be listed multiple times.

### `NetworkName=`

The (optional) name of the Podman network. If this is not specified, the default value of
`systemd-%N` is used, which is the same as the unit name but with a `systemd-` prefix to avoid
conflicts with user-managed networks.

### `Options=`

Set driver specific options.

This is equivalent to the Podman `--opt` option

### `PodmanArgs=`

This key contains a list of arguments passed directly to the end of the `podman network create` command
in the generated file (right before the name of the network in the command line). It can be used to
access Podman features otherwise unsupported by the generator. Since the generator is unaware
of what unexpected interactions can be caused by these arguments, is not recommended to use
this option.

The format of this is a space separated list of arguments, which can optionally be individually
escaped to allow inclusion of whitespace and other control characters.

This key can be listed multiple times.

### `Subnet=`

The subnet in CIDR notation.

This is equivalent to the Podman `--subnet` option

This key can be listed multiple times.

## Volume units [Volume]

Volume files are named with a `.volume` extension and contain a section `[Volume]` describing the
named Podman volume. The generated service is a one-time command that ensures that the volume
exists on the host, creating it if needed.

By default, the Podman volume has the same name as the unit, but with a `systemd-` prefix, i.e. for
a volume file named `$NAME.volume`, the generated Podman volume is called `systemd-$NAME`, and the
generated service file is `$NAME-volume.service`. The `VolumeName` option allows for overriding this
default name with a user-provided one.

Using volume units allows containers to depend on volumes being automatically pre-created. This is
particularly interesting when using special options to control volume creation,
as Podman otherwise creates volumes with the default options.

Valid options for `[Volume]` are listed below:

| **[Volume] options**      | **podman volume create equivalent** |
|---------------------------|-------------------------------------|
| Device=tmpfs              | --opt device=tmpfs                  |
| Copy=true                 | --opt copy                          |
| Group=192                 | --opt group=192                     |
| Label="foo=bar"           | --label "foo=bar"                   |
| Options=XYZ               | --opt XYZ                           |
| PodmanArgs=--driver=image | --driver=image                      |
| VolumeName=foo            | podman volume create foo            |

Supported keys in `[Volume]` section are:

### `Copy=` (default to `yes`)

If enabled, the content of the image located at the mountpoint of the volume is copied into the
volume on the first run.

### `Device=`

The path of a device which is mounted for the volume.

### `Group=`

The host (numeric) GID, or group name to use as the group for the volume

### `Label=`

Set one or more OCI labels on the volume. The format is a list of
`key=value` items, similar to `Environment`.

This key can be listed multiple times.

### `Options=`

The mount options to use for a filesystem as used by the **mount(8)** command `-o` option.

### `PodmanArgs=`

This key contains a list of arguments passed directly to the end of the `podman volume create` command
in the generated file (right before the name of the network in the command line). It can be used to
access Podman features otherwise unsupported by the generator. Since the generator is unaware
of what unexpected interactions can be caused by these arguments, is not recommended to use
this option.

The format of this is a space separated list of arguments, which can optionally be individually
escaped to allow inclusion of whitespace and other control characters.

This key can be listed multiple times.

### `Type=`

The filesystem type of `Device` as used by the **mount(8)** commands `-t` option.

### `User=`

The host (numeric) UID, or user name to use as the owner for the volume

### `VolumeName=`

The (optional) name of the Podman volume. If this is not specified, the default value of
`systemd-%N` is used, which is the same as the unit name but with a `systemd-` prefix to avoid
conflicts with user-managed volumes.

## EXAMPLES

Example `test.container`:

```
[Unit]
Description=A minimal container

[Container]
# Use the centos image
Image=quay.io/centos/centos:latest

# Use volume and network defined below
Volume=test.volume:/data
Network=test.network

# In the container we just run sleep
Exec=sleep 60

[Service]
# Restart service when sleep finishes
Restart=always

[Install]
# Start by default on boot
WantedBy=multi-user.target default.target
```

Example `test.kube`:
```
[Unit]
Description=A kubernetes yaml based service
Before=local-fs.target

[Kube]
Yaml=/opt/k8s/deployment.yml

[Install]
# Start by default on boot
WantedBy=multi-user.target default.target
```

Example `test.volume`:

```
[Volume]
User=root
Group=root
Label=org.test.Key=value
```

Example `test.network`:
```
[Network]
Subnet=172.16.0.0/24
Gateway=172.16.0.1
IPRange=172.16.0.0/28
Label=org.test.Key=value
```

## SEE ALSO
**[systemd.unit(5)](https://www.freedesktop.org/software/systemd/man/systemd.unit.html)**,
**[systemd.service(5)](https://www.freedesktop.org/software/systemd/man/systemd.service.html)**,
**[podman-run(1)](podman-run.1.md)**,
**[podman-network-create(1)](podman-network-create.1.md)**,
**[podman-auto-update(1)](podman-auto-update.1.md)**
