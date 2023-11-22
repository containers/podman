% podman-farm-build 1

## NAME
podman\-farm\-build - Build images on farm nodes, then bundle them into a manifest list

## SYNOPSIS
**podman farm build** [*options*] [*context*]

## DESCRIPTION
**podman farm build** Builds an image on all nodes in a farm and bundles them up into a manifest list.
It executes the `podman build` command on the nodes in the farm with the given Containerfile.

The manifest list will contain an image per native architecture type that is present in the farm.

The primary function of this command is to create multi-architecture builds that will be faster than doing it
via emulation using `podman build --arch --platform`.

If no farm is specified, the build will be sent out to all the nodes that `podman system connection` knows of.

## OPTIONS


[//]: # (BEGIN included file options/annotation.image.md)
#### **--annotation**=*annotation=value*

Add an image *annotation* (e.g. annotation=*value*) to the image metadata. Can
be used multiple times.

Note: this information is not present in Docker image formats, so it is
discarded when writing images in Docker formats.

[//]: # (END   included file options/annotation.image.md)


[//]: # (BEGIN included file options/authfile.md)
#### **--authfile**=*path*

Path of the authentication file. Default is `${XDG_RUNTIME_DIR}/containers/auth.json` on Linux, and `$HOME/.config/containers/auth.json` on Windows/macOS.
The file is created by **[podman login](podman-login.1.md)**. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using **docker login**.

Note: There is also the option to override the default path of the authentication file by setting the `REGISTRY_AUTH_FILE` environment variable. This can be done with **export REGISTRY_AUTH_FILE=_path_**.

[//]: # (END   included file options/authfile.md)


[//]: # (BEGIN included file options/build-arg.md)
#### **--build-arg**=*arg=value*

Specifies a build argument and its value, which is interpolated in
instructions read from the Containerfiles in the same way that environment variables are, but which are not added to environment variable list in the resulting image's configuration.

[//]: # (END   included file options/build-arg.md)


[//]: # (BEGIN included file options/build-arg-file.md)
#### **--build-arg-file**=*path*

Specifies a file containing lines of build arguments of the form `arg=value`.
The suggested file name is `argfile.conf`.

Comment lines beginning with `#` are ignored, along with blank lines.
All others must be of the `arg=value` format passed to `--build-arg`.

If several arguments are provided via the `--build-arg-file`
and `--build-arg` options, the build arguments are merged across all
of the provided files and command line arguments.

Any file provided in a `--build-arg-file` option is read before
the arguments supplied via the `--build-arg` option.

When a given argument name is specified several times, the last instance
is the one that is passed to the resulting builds. This means `--build-arg`
values always override those in a `--build-arg-file`.

[//]: # (END   included file options/build-arg-file.md)


[//]: # (BEGIN included file options/build-context.md)
#### **--build-context**=*name=value*

Specify an additional build context using its short name and its location.
Additional build contexts can be referenced in the same manner as we access
different stages in COPY instruction.

Valid values are:

* Local directory – e.g. --build-context project2=../path/to/project2/src (This option is not available with the remote Podman client. On Podman machine setup (i.e macOS and Winows) path must exists on the machine VM)
* HTTP URL to a tarball – e.g. --build-context src=https://example.org/releases/src.tar
* Container image – specified with a container-image:// prefix, e.g. --build-context alpine=container-image://alpine:3.15, (also accepts docker://, docker-image://)

On the Containerfile side, reference the build context on all
commands that accept the “from” parameter. Here’s how that might look:

```dockerfile
FROM [name]
COPY --from=[name] ...
RUN --mount=from=[name] …
```

The value of [name] is matched with the following priority order:

* Named build context defined with --build-context [name]=..
* Stage defined with AS [name] inside Containerfile
* Image [name], either local or in a remote registry

[//]: # (END   included file options/build-context.md)


[//]: # (BEGIN included file options/cache-from.md)
#### **--cache-from**=*image*

Repository to utilize as a potential cache source. When specified, Buildah tries to look for
cache images in the specified repository and attempts to pull cache images instead of actually
executing the build steps locally. Buildah only attempts to pull previously cached images if they
are considered as valid cache hits.

Use the `--cache-to` option to populate a remote repository with cache content.

Example

```bash
# populate a cache and also consult it
buildah build -t test --layers --cache-to registry/myrepo/cache --cache-from registry/myrepo/cache .
```

Note: `--cache-from` option is ignored unless `--layers` is specified.

[//]: # (END   included file options/cache-from.md)


[//]: # (BEGIN included file options/cache-to.md)
#### **--cache-to**=*image*

Set this flag to specify a remote repository that is used to store cache images. Buildah attempts to
push newly built cache image to the remote repository.

Note: Use the `--cache-from` option in order to use cache content in a remote repository.

Example

```bash
# populate a cache and also consult it
buildah build -t test --layers --cache-to registry/myrepo/cache --cache-from registry/myrepo/cache .
```

Note: `--cache-to` option is ignored unless `--layers` is specified.

[//]: # (END   included file options/cache-to.md)


[//]: # (BEGIN included file options/cache-ttl.md)
#### **--cache-ttl**

Limit the use of cached images to only consider images with created timestamps less than *duration* ago.
For example if `--cache-ttl=1h` is specified, Buildah considers intermediate cache images which are created
under the duration of one hour, and intermediate cache images outside this duration is ignored.

Note: Setting `--cache-ttl=0` manually is equivalent to using `--no-cache` in the
implementation since this means that the user dones not want to use cache at all.

[//]: # (END   included file options/cache-ttl.md)


[//]: # (BEGIN included file options/cap-add.image.md)
#### **--cap-add**=*CAP\_xxx*

When executing RUN instructions, run the command specified in the instruction
with the specified capability added to its capability set.
Certain capabilities are granted by default; this option can be used to add
more.

[//]: # (END   included file options/cap-add.image.md)


[//]: # (BEGIN included file options/cap-drop.image.md)
#### **--cap-drop**=*CAP\_xxx*

When executing RUN instructions, run the command specified in the instruction
with the specified capability removed from its capability set.
The CAP\_CHOWN, CAP\_DAC\_OVERRIDE, CAP\_FOWNER,
CAP\_FSETID, CAP\_KILL, CAP\_NET\_BIND\_SERVICE, CAP\_SETFCAP,
CAP\_SETGID, CAP\_SETPCAP, and CAP\_SETUID capabilities are
granted by default; this option can be used to remove them.

If a capability is specified to both the **--cap-add** and **--cap-drop**
options, it is dropped, regardless of the order in which the options were
given.

[//]: # (END   included file options/cap-drop.image.md)


[//]: # (BEGIN included file options/cert-dir.md)
#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry. (Default: /etc/containers/certs.d)
For details, see **[containers-certs.d(5)](https://github.com/containers/image/blob/main/docs/containers-certs.d.5.md)**.
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

[//]: # (END   included file options/cert-dir.md)


[//]: # (BEGIN included file options/cgroup-parent.md)
#### **--cgroup-parent**=*path*

Path to cgroups under which the cgroup for the container is created. If the
path is not absolute, the path is considered to be relative to the cgroups path
of the init process. Cgroups are created if they do not already exist.

[//]: # (END   included file options/cgroup-parent.md)


[//]: # (BEGIN included file options/cgroupns.image.md)
#### **--cgroupns**=*how*

Sets the configuration for cgroup namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "private" to indicate
that a new cgroup namespace is created, or it can be "host" to indicate
that the cgroup namespace in which `buildah` itself is being run is reused.

[//]: # (END   included file options/cgroupns.image.md)

#### **--cleanup**

Remove built images from farm nodes on success (Default: false).


[//]: # (BEGIN included file options/cpp-flag.md)
#### **--cpp-flag**=*flags*

Set additional flags to pass to the C Preprocessor cpp(1). Containerfiles ending with a ".in" suffix is preprocessed via cpp(1). This option can be used to pass additional flags to cpp.Note: You can also set default CPPFLAGS by setting the BUILDAH_CPPFLAGS environment variable (e.g., export BUILDAH_CPPFLAGS="-DDEBUG").

[//]: # (END   included file options/cpp-flag.md)


[//]: # (BEGIN included file options/cpu-period.md)
#### **--cpu-period**=*limit*

Set the CPU period for the Completely Fair Scheduler (CFS), which is a
duration in microseconds. Once the container's CPU quota is used up, it will not
be scheduled to run until the current period ends. Defaults to 100000
microseconds.

On some systems, changing the resource limits may not be allowed for non-root
users. For more details, see
https://github.com/containers/podman/blob/main/troubleshooting.md#26-running-containers-with-resource-limits-fails-with-a-permissions-error

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/cpu-period.md)


[//]: # (BEGIN included file options/cpu-quota.md)
#### **--cpu-quota**=*limit*

Limit the CPU Completely Fair Scheduler (CFS) quota.

Limit the container's CPU usage. By default, containers run with the full
CPU resource. The limit is a number in microseconds. If a number is provided,
the container is allowed to use that much CPU time until the CPU period
ends (controllable via **--cpu-period**).

On some systems, changing the resource limits may not be allowed for non-root
users. For more details, see
https://github.com/containers/podman/blob/main/troubleshooting.md#26-running-containers-with-resource-limits-fails-with-a-permissions-error

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/cpu-quota.md)


[//]: # (BEGIN included file options/cpu-shares.md)
#### **--cpu-shares**, **-c**=*shares*

CPU shares (relative weight).

By default, all containers get the same proportion of CPU cycles. This
proportion can be modified by changing the container's CPU share weighting
relative to the combined weight of all the running containers.
Default weight is **1024**.

The proportion only applies when CPU-intensive processes are running.
When tasks in one container are idle, other containers can use the
left-over CPU time. The actual amount of CPU time varies depending on
the number of containers running on the system.

For example, consider three containers, one has a cpu-share of 1024 and
two others have a cpu-share setting of 512. When processes in all three
containers attempt to use 100% of CPU, the first container receives
50% of the total CPU time. If a fourth container is added with a cpu-share
of 1024, the first container only gets 33% of the CPU. The remaining containers
receive 16.5%, 16.5% and 33% of the CPU.

On a multi-core system, the shares of CPU time are distributed over all CPU
cores. Even if a container is limited to less than 100% of CPU time, it can
use 100% of each individual CPU core.

For example, consider a system with more than three cores.
If the container _C0_ is started with **--cpu-shares=512** running one process,
and another container _C1_ with **--cpu-shares=1024** running two processes,
this can result in the following division of CPU shares:

| PID  |  container  | CPU     | CPU share    |
| ---- | ----------- | ------- | ------------ |
| 100  |  C0         | 0       | 100% of CPU0 |
| 101  |  C1         | 1       | 100% of CPU1 |
| 102  |  C1         | 2       | 100% of CPU2 |

On some systems, changing the resource limits may not be allowed for non-root
users. For more details, see
https://github.com/containers/podman/blob/main/troubleshooting.md#26-running-containers-with-resource-limits-fails-with-a-permissions-error

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/cpu-shares.md)


[//]: # (BEGIN included file options/cpuset-cpus.md)
#### **--cpuset-cpus**=*number*

CPUs in which to allow execution. Can be specified as a comma-separated list
(e.g. **0,1**), as a range (e.g. **0-3**), or any combination thereof
(e.g. **0-3,7,11-15**).

On some systems, changing the resource limits may not be allowed for non-root
users. For more details, see
https://github.com/containers/podman/blob/main/troubleshooting.md#26-running-containers-with-resource-limits-fails-with-a-permissions-error

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/cpuset-cpus.md)


[//]: # (BEGIN included file options/cpuset-mems.md)
#### **--cpuset-mems**=*nodes*

Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on
NUMA systems.

If there are four memory nodes on the system (0-3), use **--cpuset-mems=0,1**
then processes in the container only uses memory from the first
two memory nodes.

On some systems, changing the resource limits may not be allowed for non-root
users. For more details, see
https://github.com/containers/podman/blob/main/troubleshooting.md#26-running-containers-with-resource-limits-fails-with-a-permissions-error

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/cpuset-mems.md)


[//]: # (BEGIN included file options/creds.md)
#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry, if required.
If one or both values are not supplied, a command line prompt appears and the
value can be entered. The password is entered without echo.

Note that the specified credentials are only used to authenticate against
target registries.  They are not used for mirrors or when the registry gets
rewritten (see `containers-registries.conf(5)`); to authenticate against those
consider using a `containers-auth.json(5)` file.

[//]: # (END   included file options/creds.md)


[//]: # (BEGIN included file options/decryption-key.md)
#### **--decryption-key**=*key[:passphrase]*

The [key[:passphrase]] to be used for decryption of images. Key can point to keys and/or certificates. Decryption is tried with all keys. If the key is protected by a passphrase, it is required to be passed in the argument and omitted otherwise.

[//]: # (END   included file options/decryption-key.md)


[//]: # (BEGIN included file options/device.md)
#### **--device**=*host-device[:container-device][:permissions]*

Add a host device to the container. Optional *permissions* parameter
can be used to specify device permissions by combining
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if *host-device* is a symbolic link then it is resolved first.
The container only stores the major and minor numbers of the host device.

Podman may load kernel modules required for using the specified
device. The devices that Podman loads modules for when necessary are:
/dev/fuse.

In rootless mode, the new device is bind mounted in the container from the host
rather than Podman creating it within the container space. Because the bind
mount retains its SELinux label on SELinux systems, the container can get
permission denied when accessing the mounted device. Modify SELinux settings to
allow containers to use all device labels via the following command:

$ sudo setsebool -P  container_use_devices=true

[//]: # (END   included file options/device.md)

Note: if the user only has access rights via a group, accessing the device
from inside a rootless container fails. The **[crun(1)](https://github.com/containers/crun/tree/main/crun.1.md)** runtime offers a
workaround for this by adding the option
**--annotation run.oci.keep_original_groups=1**.


[//]: # (BEGIN included file options/disable-compression.md)
#### **--disable-compression**, **-D**

Don't compress filesystem layers when building the image unless it is required
by the location where the image is being written.  This is the default setting,
because image layers are compressed automatically when they are pushed to
registries, and images being written to local storage only need to be
decompressed again to be stored.  Compression can be forced in all cases by
specifying **--disable-compression=false**.

[//]: # (END   included file options/disable-compression.md)


[//]: # (BEGIN included file options/dns.md)
#### **--dns**=*ipaddr*

Set custom DNS servers.

This option can be used to override the DNS
configuration passed to the container. Typically this is necessary when the
host DNS configuration is invalid for the container (e.g., **127.0.0.1**). When this
is the case the **--dns** flag is necessary for every run.

The special value **none** can be specified to disable creation of _/etc/resolv.conf_ in the container by Podman.
The _/etc/resolv.conf_ file in the image is used without changes.

[//]: # (END   included file options/dns.md)

This option cannot be combined with **--network** that is set to **none**.

Note: this option takes effect only during *RUN* instructions in the build.
It does not affect _/etc/resolv.conf_ in the final image.


[//]: # (BEGIN included file options/dns-option.image.md)
#### **--dns-option**=*option*

Set custom DNS options to be used during the build.

[//]: # (END   included file options/dns-option.image.md)


[//]: # (BEGIN included file options/dns-search.image.md)
#### **--dns-search**=*domain*

Set custom DNS search domains to be used during the build.

[//]: # (END   included file options/dns-search.image.md)


[//]: # (BEGIN included file options/env.image.md)
#### **--env**=*env[=value]*

Add a value (e.g. env=*value*) to the built image.  Can be used multiple times.
If neither `=` nor a *value* are specified, but *env* is set in the current
environment, the value from the current environment is added to the image.
To remove an environment variable from the built image, use the `--unsetenv`
option.

[//]: # (END   included file options/env.image.md)


[//]: # (BEGIN included file options/file.md)
#### **--file**, **-f**=*Containerfile*

Specifies a Containerfile which contains instructions for building the image,
either a local file or an **http** or **https** URL.  If more than one
Containerfile is specified, *FROM* instructions are only be accepted from the
last specified file.

If a build context is not specified, and at least one Containerfile is a
local file, the directory in which it resides is used as the build
context.

Specifying the option `-f -` causes the Containerfile contents to be read from stdin.

[//]: # (END   included file options/file.md)


[//]: # (BEGIN included file options/force-rm.md)
#### **--force-rm**

Always remove intermediate containers after a build, even if the build fails (default true).

[//]: # (END   included file options/force-rm.md)


[//]: # (BEGIN included file options/format.md)
#### **--format**

Control the format for the built image's manifest and configuration data.
Recognized formats include *oci* (OCI image-spec v1.0, the default) and
*docker* (version 2, using schema format 2 for the manifest).

Note: You can also override the default format by setting the BUILDAH\_FORMAT
environment variable.  `export BUILDAH_FORMAT=docker`

[//]: # (END   included file options/format.md)


[//]: # (BEGIN included file options/from.md)
#### **--from**

Overrides the first `FROM` instruction within the Containerfile.  If there are multiple
FROM instructions in a Containerfile, only the first is changed.

With the remote podman client, not all container transports work as
expected. For example, oci-archive:/x.tar references /x.tar on the remote
machine instead of on the client. When using podman remote clients it is
best to restrict use to *containers-storage*, and *docker:// transports*.

[//]: # (END   included file options/from.md)


[//]: # (BEGIN included file options/group-add.md)
#### **--group-add**=*group* | *keep-groups*

Assign additional groups to the primary user running within the container process.

- `keep-groups` is a special flag that tells Podman to keep the supplementary group access.

Allows container to use the user's supplementary group access. If file systems or
devices are only accessible by the rootless user's group, this flag tells the OCI
runtime to pass the group access into the container. Currently only available
with the `crun` OCI runtime. Note: `keep-groups` is exclusive, other groups cannot be specified
with this flag. (Not available for remote commands, including Mac and Windows (excluding WSL2) machines)

[//]: # (END   included file options/group-add.md)


[//]: # (BEGIN included file options/help.md)
#### **--help**, **-h**

Print usage statement

[//]: # (END   included file options/help.md)


[//]: # (BEGIN included file options/hooks-dir.md)
#### **--hooks-dir**=*path*

Each *.json file in the path configures a hook for buildah build containers. For more details on the syntax of the JSON files and the semantics of hook injection. Buildah currently support both the 1.0.0 and 0.1.0 hook schemas, although the 0.1.0 schema is deprecated.

This option may be set multiple times; paths from later options have higher precedence.

For the annotation conditions, buildah uses any annotations set in the generated OCI configuration.

For the bind-mount conditions, only mounts explicitly requested by the caller via --volume are considered. Bind mounts that buildah inserts by default (e.g. /dev/shm) are not considered.

If --hooks-dir is unset for root callers, Buildah currently defaults to /usr/share/containers/oci/hooks.d and /etc/containers/oci/hooks.d in order of increasing precedence. Using these defaults is deprecated. Migrate to explicitly setting --hooks-dir.

[//]: # (END   included file options/hooks-dir.md)


[//]: # (BEGIN included file options/http-proxy.md)
#### **--http-proxy**

By default proxy environment variables are passed into the container if set
for the Podman process. This can be disabled by setting the value to **false**.
The environment variables passed in include **http_proxy**,
**https_proxy**, **ftp_proxy**, **no_proxy**, and also the upper case versions of
those. This option is only needed when the host system must use a proxy but
the container does not use any proxy. Proxy environment variables specified
for the container in any other way overrides the values that have
been passed through from the host. (Other ways to specify the proxy for the
container include passing the values with the **--env** flag, or hard coding the
proxy environment at container build time.)
When used with the remote client it uses the proxy environment variables
that are set on the server process.

Defaults to **true**.

[//]: # (END   included file options/http-proxy.md)


[//]: # (BEGIN included file options/identity-label.md)
#### **--identity-label**

Adds default identity label `io.buildah.version` if set. (default true).

[//]: # (END   included file options/identity-label.md)


[//]: # (BEGIN included file options/ignorefile.md)
#### **--ignorefile**

Path to an alternative .containerignore file.

[//]: # (END   included file options/ignorefile.md)


[//]: # (BEGIN included file options/iidfile.md)
#### **--iidfile**=*ImageIDfile*

Write the built image's ID to the file.  When `--platform` is specified more than once, attempting to use this option triggers an error.

[//]: # (END   included file options/iidfile.md)


[//]: # (BEGIN included file options/ipc.image.md)
#### **--ipc**=*how*

Sets the configuration for IPC namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate
that a new IPC namespace is created, or it can be "host" to indicate
that the IPC namespace in which `podman` itself is being run is reused,
or it can be the path to an IPC namespace which is already in use by
another process.

[//]: # (END   included file options/ipc.image.md)


[//]: # (BEGIN included file options/isolation.md)
#### **--isolation**=*type*

Controls what type of isolation is used for running processes as part of `RUN`
instructions.  Recognized types include *oci* (OCI-compatible runtime, the
default), *rootless* (OCI-compatible runtime invoked using a modified
configuration and its --rootless option enabled, with *--no-new-keyring
--no-pivot* added to its *create* invocation, with network and UTS namespaces
disabled, and IPC, PID, and user namespaces enabled; the default for
unprivileged users), and *chroot* (an internal wrapper that leans more toward
chroot(1) than container technology).

Note: You can also override the default isolation type by setting the
BUILDAH\_ISOLATION environment variable.  `export BUILDAH_ISOLATION=oci`

[//]: # (END   included file options/isolation.md)


[//]: # (BEGIN included file options/jobs.md)
#### **--jobs**=*number*

Run up to N concurrent stages in parallel.  If the number of jobs is greater
than 1, stdin is read from /dev/null.  If 0 is specified, then there is
no limit in the number of jobs that run in parallel.

[//]: # (END   included file options/jobs.md)


[//]: # (BEGIN included file options/label.image.md)
#### **--label**=*label*

Add an image *label* (e.g. label=*value*) to the image metadata. Can be used
multiple times.

Users can set a special LABEL **io.containers.capabilities=CAP1,CAP2,CAP3** in
a Containerfile that specifies the list of Linux capabilities required for the
container to run properly. This label specified in a container image tells
Podman to run the container with just these capabilities. Podman launches the
container with just the specified capabilities, as long as this list of
capabilities is a subset of the default list.

If the specified capabilities are not in the default set, Podman prints an error
message and runs the container with the default capabilities.

[//]: # (END   included file options/label.image.md)


[//]: # (BEGIN included file options/layer-label.md)
#### **--layer-label**=*label[=value]*

Add an intermediate image *label* (e.g. label=*value*) to the intermediate
image metadata. It can be used multiple times.

If *label* is named, but neither `=` nor a `value` is provided, then
the *label* is set to an empty value.

[//]: # (END   included file options/layer-label.md)


[//]: # (BEGIN included file options/layers.md)
#### **--layers**

Cache intermediate images during the build process (Default is `true`).

Note: You can also override the default value of layers by setting the
BUILDAH\_LAYERS environment variable. `export BUILDAH_LAYERS=true`

[//]: # (END   included file options/layers.md)

#### **--local**

Build image on local machine as well as on farm nodes.


[//]: # (BEGIN included file options/logfile.md)
#### **--logfile**=*filename*

Log output which is sent to standard output and standard error to the
specified file instead of to standard output and standard error.
This option is not supported on the remote client, including Mac and Windows (excluding WSL2) machines.

[//]: # (END   included file options/logfile.md)


[//]: # (BEGIN included file options/memory.md)
#### **--memory**, **-m**=*number[unit]*

Memory limit. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).

Allows the memory available to a container to be constrained. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value is very large, that's millions of trillions).

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/memory.md)


[//]: # (BEGIN included file options/memory-swap.md)
#### **--memory-swap**=*number[unit]*

A limit value equal to memory plus swap.
A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).

Must be used with the **-m** (**--memory**) flag.
The argument value must be larger than that of
 **-m** (**--memory**) By default, it is set to double
the value of **--memory**.

Set _number_ to **-1** to enable unlimited swap.

This option is not supported on cgroups V1 rootless systems.

[//]: # (END   included file options/memory-swap.md)


[//]: # (BEGIN included file options/network.image.md)
#### **--network**=*mode*, **--net**

Sets the configuration for network namespaces when handling `RUN` instructions.

Valid _mode_ values are:

- **none**: no networking.
- **host**: use the Podman host network stack. Note: the host mode gives the
container full access to local system services such as D-bus and is therefore
considered insecure.
- **ns:**_path_: path to a network namespace to join.
- **private**: create a new namespace for the container (default)
- **\<network name|ID\>**: Join the network with the given name or ID, e.g. use `--network mynet` to join the network with the name mynet. Only supported for rootful users.
- **slirp4netns[:OPTIONS,...]**: use **slirp4netns**(1) to create a user network stack. This is the default for rootless containers. It is possible to specify these additional options, they can also be set with `network_cmd_options` in containers.conf:
  - **allow_host_loopback=true|false**: Allow slirp4netns to reach the host loopback IP (default is 10.0.2.2 or the second IP from slirp4netns cidr subnet when changed, see the cidr option below). The default is false.
  - **mtu=MTU**: Specify the MTU to use for this network. (Default is `65520`).
  - **cidr=CIDR**: Specify ip range to use for this network. (Default is `10.0.2.0/24`).
  - **enable_ipv6=true|false**: Enable IPv6. Default is true. (Required for `outbound_addr6`).
  - **outbound_addr=INTERFACE**: Specify the outbound interface slirp binds to (ipv4 traffic only).
  - **outbound_addr=IPv4**: Specify the outbound ipv4 address slirp binds to.
  - **outbound_addr6=INTERFACE**: Specify the outbound interface slirp binds to (ipv6 traffic only).
  - **outbound_addr6=IPv6**: Specify the outbound ipv6 address slirp binds to.
- **pasta[:OPTIONS,...]**: use **pasta**(1) to create a user-mode networking
    stack. \
    This is only supported in rootless mode. \
    By default, IPv4 and IPv6 addresses and routes, as well as the pod interface
    name, are copied from the host. If port forwarding isn't configured, ports
    are forwarded dynamically as services are bound on either side (init
    namespace or container namespace). Port forwarding preserves the original
    source IP address. Options described in pasta(1) can be specified as
    comma-separated arguments. \
    In terms of pasta(1) options, **--config-net** is given by default, in
    order to configure networking when the container is started, and
    **--no-map-gw** is also assumed by default, to avoid direct access from
    container to host using the gateway address. The latter can be overridden
    by passing **--map-gw** in the pasta-specific options (despite not being an
    actual pasta(1) option). \
    Also, **-t none** and **-u none** are passed to disable
    automatic port forwarding based on bound ports. Similarly, **-T none** and
    **-U none** are given to disable the same functionality from container to
    host. \
    Some examples:
    - **pasta:--map-gw**: Allow the container to directly reach the host using the
        gateway address.
    - **pasta:--mtu,1500**: Specify a 1500 bytes MTU for the _tap_ interface in
        the container.
    - **pasta:--ipv4-only,-a,10.0.2.0,-n,24,-g,10.0.2.2,--dns-forward,10.0.2.3,-m,1500,--no-ndp,--no-dhcpv6,--no-dhcp**,
        equivalent to default slirp4netns(1) options: disable IPv6, assign
        `10.0.2.0/24` to the `tap0` interface in the container, with gateway
        `10.0.2.3`, enable DNS forwarder reachable at `10.0.2.3`, set MTU to 1500
        bytes, disable NDP, DHCPv6 and DHCP support.
    - **pasta:-I,tap0,--ipv4-only,-a,10.0.2.0,-n,24,-g,10.0.2.2,--dns-forward,10.0.2.3,--no-ndp,--no-dhcpv6,--no-dhcp**,
        equivalent to default slirp4netns(1) options with Podman overrides: same as
        above, but leave the MTU to 65520 bytes
    - **pasta:-t,auto,-u,auto,-T,auto,-U,auto**: enable automatic port forwarding
        based on observed bound ports from both host and container sides
    - **pasta:-T,5201**: enable forwarding of TCP port 5201 from container to
        host, using the loopback interface instead of the tap interface for improved
        performance

[//]: # (END   included file options/network.image.md)


[//]: # (BEGIN included file options/no-cache.md)
#### **--no-cache**

Do not use existing cached images for the container build. Build from the start with a new set of cached layers.

[//]: # (END   included file options/no-cache.md)


[//]: # (BEGIN included file options/no-hosts.md)
#### **--no-hosts**

Do not create _/etc/hosts_ for the container.
By default, Podman manages _/etc/hosts_, adding the container's own IP address and any hosts from **--add-host**.
**--no-hosts** disables this, and the image's _/etc/hosts_ is preserved unmodified.

[//]: # (END   included file options/no-hosts.md)

This option conflicts with **--add-host**.


[//]: # (BEGIN included file options/omit-history.md)
#### **--omit-history**

Omit build history information in the built image. (default false).

This option is useful for the cases where end users explicitly
want to set `--omit-history` to omit the optional `History` from
built images or when working with images built using build tools that
do not include `History` information in their images.

[//]: # (END   included file options/omit-history.md)


[//]: # (BEGIN included file options/os-feature.md)
#### **--os-feature**=*feature*

Set the name of a required operating system *feature* for the image which is built.  By default, if the image is not based on *scratch*, the base image's required OS feature list is kept, if the base image specified any.  This option is typically only meaningful when the image's OS is Windows.

If *feature* has a trailing `-`, then the *feature* is removed from the set of required features which is listed in the image.

[//]: # (END   included file options/os-feature.md)


[//]: # (BEGIN included file options/os-version.image.md)
#### **--os-version**=*version*

Set the exact required operating system *version* for the image which is built.  By default, if the image is not based on *scratch*, the base image's required OS version is kept, if the base image specified one.  This option is typically only meaningful when the image's OS is Windows, and is typically set in Windows base images, so using this option is usually unnecessary.

[//]: # (END   included file options/os-version.image.md)


[//]: # (BEGIN included file options/pid.image.md)
#### **--pid**=*pid*

Sets the configuration for PID namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate that a new PID namespace is created, or it can be "host" to indicate that the PID namespace in which `podman` itself is being run is reused, or it can be the path to a PID namespace which is already in use by another
process.

[//]: # (END   included file options/pid.image.md)

#### **--platforms**=*p1,p2,p3...*

Build only on farm nodes that match the given platforms.


[//]: # (BEGIN included file options/pull.image.md)
#### **--pull**=*policy*

Pull image policy. The default is **always**.

- **always**, **true**: Always pull the image and throw an error if the pull fails.
- **missing**: Only pull the image when it does not exist in the local containers storage.  Throw an error if no image is found and the pull fails.
- **never**, **false**: Never pull the image but use the one from the local containers storage.  Throw an error when no image is found.
- **newer**: Pull if the image on the registry is newer than the one in the local containers storage.  An image is considered to be newer when the digests are different.  Comparing the time stamps is prone to errors.  Pull errors are suppressed if a local image was found.

[//]: # (END   included file options/pull.image.md)


[//]: # (BEGIN included file options/quiet.md)
#### **--quiet**, **-q**

Suppress output messages which indicate which instruction is being processed, and of progress when pulling images from a registry, and when writing the output image.

[//]: # (END   included file options/quiet.md)


[//]: # (BEGIN included file options/retry.md)
#### **--retry**=*attempts*

Number of times to retry in case of failure when performing pull of
images from registry. Default is **3**.

[//]: # (END   included file options/retry.md)


[//]: # (BEGIN included file options/retry-delay.md)
#### **--retry-delay**=*duration*

Duration of delay between retry attempts in case of failure when performing pull of images from registry. Default is **2s**.

[//]: # (END   included file options/retry-delay.md)


[//]: # (BEGIN included file options/rm.md)
#### **--rm**

Remove intermediate containers after a successful build (default true).

[//]: # (END   included file options/rm.md)


[//]: # (BEGIN included file options/runtime.md)
#### **--runtime**=*path*

The *path* to an alternate OCI-compatible runtime, which is used to run
commands specified by the **RUN** instruction.

Note: You can also override the default runtime by setting the BUILDAH\_RUNTIME environment variable.  `export BUILDAH_RUNTIME=/usr/local/bin/runc`

[//]: # (END   included file options/runtime.md)


[//]: # (BEGIN included file options/runtime-flag.md)
#### **--runtime-flag**=*flag*

Adds global flags for the container rutime. To list the supported flags, please consult the manpages of the selected container runtime.

Note: Do not pass the leading -- to the flag. To pass the runc flag --log-format json to buildah build, the option given is --runtime-flag log-format=json.

[//]: # (END   included file options/runtime-flag.md)


[//]: # (BEGIN included file options/secret.image.md)
#### **--secret**=**id=id,src=path**

Pass secret information used in the Containerfile for building images
in a safe way that are not stored in the final image, or be seen in other stages.
The secret is mounted in the container at the default location of `/run/secrets/id`.

To later use the secret, use the --mount option in a `RUN` instruction within a `Containerfile`:

`RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret`

[//]: # (END   included file options/secret.image.md)


[//]: # (BEGIN included file options/security-opt.image.md)
#### **--security-opt**=*option*

Security Options

- `apparmor=unconfined` : Turn off apparmor confinement for the container
- `apparmor=alternate-profile` : Set the apparmor confinement profile for the
container

- `label=user:USER`     : Set the label user for the container processes
- `label=role:ROLE`     : Set the label role for the container processes
- `label=type:TYPE`     : Set the label process type for the container processes
- `label=level:LEVEL`   : Set the label level for the container processes
- `label=filetype:TYPE` : Set the label file type for the container files
- `label=disable`       : Turn off label separation for the container
- `no-new-privileges`   : Not supported

- `seccomp=unconfined` : Turn off seccomp confinement for the container
- `seccomp=profile.json` :  White listed syscalls seccomp Json file to be used as a seccomp filter

[//]: # (END   included file options/security-opt.image.md)


[//]: # (BEGIN included file options/shm-size.md)
#### **--shm-size**=*number[unit]*

Size of _/dev/shm_. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).
If the unit is omitted, the system uses bytes. If the size is omitted, the default is **64m**.
When _size_ is **0**, there is no limit on the amount of memory used for IPC by the container.
This option conflicts with **--ipc=host**.

[//]: # (END   included file options/shm-size.md)


[//]: # (BEGIN included file options/skip-unused-stages.md)
#### **--skip-unused-stages**

Skip stages in multi-stage builds which don't affect the target stage. (Default: **true**).

[//]: # (END   included file options/skip-unused-stages.md)


[//]: # (BEGIN included file options/squash.md)
#### **--squash**

Squash all of the image's new layers into a single new layer; any preexisting layers are not squashed.

[//]: # (END   included file options/squash.md)


[//]: # (BEGIN included file options/squash-all.md)
#### **--squash-all**

Squash all of the new image's layers (including those inherited from a base image) into a single new layer.

[//]: # (END   included file options/squash-all.md)


[//]: # (BEGIN included file options/ssh.md)
#### **--ssh**=*default* | *id[=socket>*

SSH agent socket or keys to expose to the build.
The socket path can be left empty to use the value of `default=$SSH_AUTH_SOCK`

To later use the ssh agent, use the --mount option in a `RUN` instruction within a `Containerfile`:

`RUN --mount=type=ssh,id=id mycmd`

[//]: # (END   included file options/ssh.md)


[//]: # (BEGIN included file options/tag.md)
#### **--tag**, **-t**=*imageName*

Specifies the name which is assigned to the resulting image if the build process completes successfully.
If _imageName_ does not include a registry name, the registry name *localhost* is prepended to the image name.

[//]: # (END   included file options/tag.md)


[//]: # (BEGIN included file options/target.md)
#### **--target**=*stageName*

Set the target build stage to build.  When building a Containerfile with multiple build stages, --target can be used to specify an intermediate build stage by name as the final stage for the resulting image. Commands after the target stage is skipped.

[//]: # (END   included file options/target.md)


[//]: # (BEGIN included file options/timestamp.md)
#### **--timestamp**=*seconds*

Set the create timestamp to seconds since epoch to allow for deterministic builds (defaults to current time). By default, the created timestamp is changed and written into the image manifest with every commit, causing the image's sha256 hash to be different even if the sources are exactly the same otherwise.
When --timestamp is set, the created timestamp is always set to the time specified and therefore not changed, allowing the image's sha256 hash to remain the same. All files committed to the layers of the image is created with the timestamp.

If the only instruction in a Containerfile is `FROM`, this flag has no effect.

[//]: # (END   included file options/timestamp.md)


[//]: # (BEGIN included file options/ulimit.image.md)
#### **--ulimit**=*type=soft-limit[:hard-limit]*

Specifies resource limits to apply to processes launched when processing `RUN` instructions. This option can be specified multiple times.  Recognized resource types include:
  "core": maximum core dump size (ulimit -c)
  "cpu": maximum CPU time (ulimit -t)
  "data": maximum size of a process's data segment (ulimit -d)
  "fsize": maximum size of new files (ulimit -f)
  "locks": maximum number of file locks (ulimit -x)
  "memlock": maximum amount of locked memory (ulimit -l)
  "msgqueue": maximum amount of data in message queues (ulimit -q)
  "nice": niceness adjustment (nice -n, ulimit -e)
  "nofile": maximum number of open files (ulimit -n)
  "nproc": maximum number of processes (ulimit -u)
  "rss": maximum size of a process's (ulimit -m)
  "rtprio": maximum real-time scheduling priority (ulimit -r)
  "rttime": maximum amount of real-time execution between blocking syscalls
  "sigpending": maximum number of pending signals (ulimit -i)
  "stack": maximum stack size (ulimit -s)

[//]: # (END   included file options/ulimit.image.md)


[//]: # (BEGIN included file options/unsetenv.image.md)
#### **--unsetenv**=*env*

Unset environment variables from the final image.

[//]: # (END   included file options/unsetenv.image.md)


[//]: # (BEGIN included file options/unsetlabel.md)
#### **--unsetlabel**=*label*

Unset the image label, causing the label not to be inherited from the base image.

[//]: # (END   included file options/unsetlabel.md)


[//]: # (BEGIN included file options/userns.image.md)
#### **--userns**=*how*

Sets the configuration for user namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate that a new user namespace is created, it can be "host" to indicate that the user namespace in which `podman` itself is being run is reused, or it can be the path to a user namespace which is already in use by another process.

[//]: # (END   included file options/userns.image.md)


[//]: # (BEGIN included file options/userns-gid-map.md)
#### **--userns-gid-map**=*mapping*

Directly specifies a GID mapping to be used to set ownership, at the
filesystem level, on the working container's contents.
Commands run when handling `RUN` instructions defaults to being run in
their own user namespaces, configured using the UID and GID maps.

Entries in this map take the form of one or more triples of a starting
in-container GID, a corresponding starting host-level GID, and the number of consecutive IDs which the map entry represents.

This option overrides the *remap-gids* setting in the *options* section of /etc/containers/storage.conf.

If this option is not specified, but a global --userns-gid-map setting is supplied, settings from the global option is used.

If none of --userns-uid-map-user, --userns-gid-map-group, or --userns-gid-map are specified, but --userns-uid-map is specified, the GID map is set to use the same numeric values as the UID map.

[//]: # (END   included file options/userns-gid-map.md)


[//]: # (BEGIN included file options/userns-gid-map-group.md)
#### **--userns-gid-map-group**=*group*

Specifies that a GID mapping to be used to set ownership, at the
filesystem level, on the working container's contents, can be found in entries in the `/etc/subgid` file which correspond to the specified group.
Commands run when handling `RUN` instructions defaults to being run in
their own user namespaces, configured using the UID and GID maps.
If --userns-uid-map-user is specified, but --userns-gid-map-group is not specified, `podman` assumes that the specified user name is also a
suitable group name to use as the default setting for this option.

**NOTE:** When this option is specified by a rootless user, the specified mappings are relative to the rootless user namespace in the container, rather than being relative to the host as it is when run rootful.

[//]: # (END   included file options/userns-gid-map-group.md)


[//]: # (BEGIN included file options/userns-uid-map.md)
#### **--userns-uid-map**=*mapping*

Directly specifies a UID mapping to be used to set ownership, at the
filesystem level, on the working container's contents.
Commands run when handling `RUN` instructions default to being run in
their own user namespaces, configured using the UID and GID maps.

Entries in this map take the form of one or more triples of a starting
in-container UID, a corresponding starting host-level UID, and the number of consecutive IDs which the map entry represents.

This option overrides the *remap-uids* setting in the *options* section of /etc/containers/storage.conf.

If this option is not specified, but a global --userns-uid-map setting is supplied, settings from the global option is used.

If none of --userns-uid-map-user, --userns-gid-map-group, or --userns-uid-map are specified, but --userns-gid-map is specified, the UID map is set to use the same numeric values as the GID map.

[//]: # (END   included file options/userns-uid-map.md)


[//]: # (BEGIN included file options/userns-uid-map-user.md)
#### **--userns-uid-map-user**=*user*

Specifies that a UID mapping to be used to set ownership, at the
filesystem level, on the working container's contents, can be found in entries in the `/etc/subuid` file which correspond to the specified user.
Commands run when handling `RUN` instructions defaults to being run in
their own user namespaces, configured using the UID and GID maps.
If --userns-gid-map-group is specified, but --userns-uid-map-user is not specified, `podman` assumes that the specified group name is also a
suitable user name to use as the default setting for this option.

**NOTE:** When this option is specified by a rootless user, the specified mappings are relative to the rootless user namespace in the container, rather than being relative to the host as it is when run rootful.

[//]: # (END   included file options/userns-uid-map-user.md)


[//]: # (BEGIN included file options/uts.md)
#### **--uts**=*how*

Sets the configuration for UTS namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate that a new UTS namespace to be created, or it can be "host" to indicate that the UTS namespace in which `podman` itself is being run is reused, or it can be the path to a UTS namespace which is already in use by another process.

[//]: # (END   included file options/uts.md)


[//]: # (BEGIN included file options/volume.image.md)
#### **--volume**, **-v**=*[HOST-DIR:CONTAINER-DIR[:OPTIONS]]*

Create a bind mount. Specifying the `-v /HOST-DIR:/CONTAINER-DIR` option, Podman
bind mounts `/HOST-DIR` from the host to `/CONTAINER-DIR` in the Podman
container.

The `OPTIONS` are a comma-separated list and can be: <sup>[[1]](#Footnote1)</sup>

   * [rw|ro]
   * [z|Z|O]
   * [U]
   * [`[r]shared`|`[r]slave`|`[r]private`]

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The `HOST-DIR`
must be an absolute path as well. Podman bind-mounts the `HOST-DIR` to the
specified path. For example, when specifying the host path `/foo`,
Podman copies the contents of `/foo` to the container filesystem on the host
and bind mounts that into the container.

You can specify multiple  **-v** options to mount one or more mounts to a
container.

You can add the `:ro` or `:rw` suffix to a volume to mount it read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

  `Chowning Volume Mounts`

By default, Podman does not change the owner and group of source volume
directories mounted. When running using user namespaces, the UID and GID inside
the namespace may correspond to another UID and GID on the host.

The `:U` suffix tells Podman to use the correct host UID and GID based on the
UID and GID within the namespace, to change recursively the owner and group of
the source volume.

**Warning** use with caution since this modifies the host filesystem.

  `Labeling Volume Mounts`

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the container context, add one of these two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The `z` option tells Podman that two containers
share the volume content. As a result, Podman labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Podman to label the content with a private unshared label.
Only the current container can use a private volume.

Note: Do not relabel system files and directories. Relabeling system content
might cause other confined services on the host machine to fail.  For these types
of containers, disabling SELinux separation is recommended.  The option
`--security-opt label=disable` disables SELinux separation for the container.
For example, if a user wanted to volume mount their entire home directory into the build containers, they need to disable SELinux separation.

	   $ podman build --security-opt label=disable -v $HOME:/home/user .

  `Overlay Volume Mounts`

   The `:O` flag tells Podman to mount the directory from the host as a
temporary storage using the Overlay file system. The `RUN` command containers
are allowed to modify contents within the mountpoint and are stored in the
container storage in a separate directory.  In Overlay FS terms the source
directory is the lower, and the container storage directory is the
upper. Modifications to the mount point are destroyed when the `RUN` command
finishes executing, similar to a tmpfs mount point.

  Any subsequent execution of `RUN` commands sees the original source directory
content, any changes from previous RUN commands no longer exists.

  One use case of the `overlay` mount is sharing the package cache from the
host into the container to allow speeding up builds.

  Note:

     - Overlay mounts are not currently supported in rootless mode.
     - The `O` flag is not allowed to be specified with the `Z` or `z` flags.
Content mounted into the container is labeled with the private label.
       On SELinux systems, labels in the source directory needs to be readable
by the container label. If not, SELinux container separation must be disabled
for the container to work.
     - Modification of the directory volume mounted into the container with an
overlay mount can cause unexpected failures. Do not modify the directory until
the container finishes running.

By default bind mounted volumes are `private`. That means any mounts done
inside containers are not be visible on the host and vice versa. This behavior
can be changed by specifying a volume mount propagation property.

When the mount propagation policy is set to `shared`, any mounts completed
inside the container on that volume is visible to both the host and
container. When the mount propagation policy is set to `slave`, one way mount
propagation is enabled and any mounts completed on the host for that volume is
visible only inside of the container. To control the mount propagation
property of volume use the `:[r]shared`, `:[r]slave` or `:[r]private`
propagation flag. For mount propagation to work on the source mount point (mount
point where source dir is mounted on) has to have the right propagation properties.
For shared volumes, the source mount point has to be shared. And for slave volumes,
the source mount has to be either shared or slave. <sup>[[1]](#Footnote1)</sup>

Use `df <source-dir>` to determine the source mount and then use
`findmnt -o TARGET,PROPAGATION <source-mount-dir>` to determine propagation
properties of source mount, if `findmnt` utility is not available, the source
mount point can be determined by looking at the mount entry in
`/proc/self/mountinfo`. Look at `optional fields` and see if any propagation
properties are specified.
`shared:X` means the mount is `shared`, `master:X` means the mount is `slave`
and if nothing is there that means the mount is `private`. <sup>[[1]](#Footnote1)</sup>

To change propagation properties of a mount point use the `mount` command. For
example, to bind mount the source directory `/foo` do
`mount --bind /foo /foo` and `mount --make-private --make-shared /foo`. This
converts /foo into a `shared` mount point.  The propagation properties of
the source mount can be changed directly. For instance if `/` is the source
mount for `/foo`, then use `mount --make-shared /` to convert `/` into a
`shared` mount.

[//]: # (END   included file options/volume.image.md)

## EXAMPLES

```
$ podman farm build --local -t name -f /path/to/containerfile .

$ podman farm --farm build myfarm -t name .

$ podman farm --farm myfarm build --cleanup -t name .

$ podman farm build --platforms arm64,amd64 --cleanup -t name .
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-farm(1)](podman-farm.1.md)**, **[buildah(1)](https://github.com/containers/buildah/blob/main/docs/buildah.1.md)**, **[containers-certs.d(5)](https://github.com/containers/image/blob/main/docs/containers-certs.d.5.md)**, **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**, **[crun(1)](https://github.com/containers/crun/blob/main/crun.1.md)**, **[runc(8)](https://github.com/opencontainers/runc/blob/main/man/runc.8.md)**, **[useradd(8)](https://www.unix.com/man-page/redhat/8/useradd)**, **[Containerfile(5)](https://github.com/containers/common/blob/main/docs/Containerfile.5.md)**, **[containerignore(5)](https://github.com/containers/common/blob/main/docs/containerignore.5.md)**

## HISTORY

September 2023, Originally compiled by Urvashi Mohnani `<umohnani@redhat.com>`

## FOOTNOTES
<a name="Footnote1">1</a>: The Podman project is committed to inclusivity, a
core value of open source. The `master` and `slave` mount propagation
terminology used here is problematic and divisive, and needs to be changed.
However, these terms are currently used within the Linux kernel and must be
used as-is at this time. When the kernel maintainers rectify this usage,
Podman will follow suit immediately.
