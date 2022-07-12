% podman-pod-clone(1)

## NAME
podman\-pod\-clone - Creates a copy of an existing pod

## SYNOPSIS
**podman pod clone** [*options*] *pod* *name*

## DESCRIPTION
**podman pod clone** creates a copy of a pod, recreating the identical config for the pod and for all of its containers. Users can modify the pods new name and select pod details within the infra container

## OPTIONS

#### **--cgroup-parent**=*path*

Path to cgroups under which the cgroup for the pod will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

#### **--cpus**

Set a number of CPUs for the pod that overrides the original pods CPU limits. If none are specified, the original pod's Nano CPUs are used.

#### **--cpuset-cpus**

CPUs in which to allow execution (0-3, 0,1). If none are specified, the original pod's CPUset is used.

#### **--destroy**

Remove the original pod that we are cloning once used to mimic the configuration.

#### **--device**=*host-device[:container-device][:permissions]*

Add a host device to the pod. Optional *permissions* parameter
can be used to specify device permissions. It is a combination of
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if _host_device_ is a symbolic link then it will be resolved first.
The pod will only store the major and minor numbers of the host device.

Note: the pod implements devices by storing the initial configuration passed by the user and recreating the device on each container added to the pod.

Podman may load kernel modules required for using the specified
device. The devices that Podman will load modules for when necessary are:
/dev/fuse.

#### **--device-read-bps**=*path*

Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb).

#### **--gidmap**=*pod_gid:host_gid:amount*

GID map for the user namespace. Using this flag will run all containers in the pod with user namespace enabled. It conflicts with the `--userns` and `--subgidname` flags.

#### **--help**, **-h**

Print usage statement.

#### **--hostname**=*name*

Set a hostname to the pod.

#### **--infra-command**=*command*

The command that will be run to start the infra container. Default: "/pause".

#### **--infra-conmon-pidfile**=*file*

Write the pid of the infra container's **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to manage Podman containers and pods.

#### **--infra-name**=*name*

The name that will be used for the pod's infra container.

#### **--label**, **-l**=*label*

Add metadata to a pod (e.g., --label com.example.key=value).

#### **--label-file**=*label*

Read in a line delimited file of labels.

#### **--memory**, **-m**=*limit*

Memory limit (format: `<number>[<unit>]`, where unit = b (bytes), k (kibibytes), m (mebibytes), or g (gibibytes))

Constrains the memory available to a container. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

#### **--name**, **-n**

Set a custom name for the cloned pod. The default if not specified is of the syntax: **<ORIGINAL_NAME>-clone**

#### **--pid**=*pid*

Set the PID mode for the pod. The default is to create a private PID namespace for the pod. Requires the PID namespace to be shared via --share.

    host: use the host’s PID namespace for the pod
    ns: join the specified PID namespace
    private: create a new namespace for the pod (default)

#### **--security-opt**=*option*

Security Options

- `apparmor=unconfined` : Turn off apparmor confinement for the pod
- `apparmor=your-profile` : Set the apparmor confinement profile for the pod

- `label=user:USER`     : Set the label user for the pod processes
- `label=role:ROLE`     : Set the label role for the pod processes
- `label=type:TYPE`     : Set the label process type for the pod processes
- `label=level:LEVEL`   : Set the label level for the pod processes
- `label=filetype:TYPE` : Set the label file type for the pod files
- `label=disable`       : Turn off label separation for the pod

Note: Labeling can be disabled for all pods/containers by setting label=false in the **containers.conf** (`/etc/containers/containers.conf` or `$HOME/.config/containers/containers.conf`) file.

- `mask=/path/1:/path/2` : The paths to mask separated by a colon. A masked path
  cannot be accessed inside the containers within the pod.

- `no-new-privileges` : Disable container processes from gaining additional privileges.

- `seccomp=unconfined` : Turn off seccomp confinement for the pod
- `seccomp=profile.json` :  Whitelisted syscalls seccomp Json file to be used as a seccomp filter

- `proc-opts=OPTIONS` : Comma-separated list of options to use for the /proc mount. More details for the
  possible mount options are specified in the **proc(5)** man page.

- **unmask**=_ALL_ or _/path/1:/path/2_, or shell expanded paths (/proc/*): Paths to unmask separated by a colon. If set to **ALL**, it will unmask all the paths that are masked or made read-only by default.
  The default masked paths are **/proc/acpi, /proc/kcore, /proc/keys, /proc/latency_stats, /proc/sched_debug, /proc/scsi, /proc/timer_list, /proc/timer_stats, /sys/firmware, and /sys/fs/selinux.**  The default paths that are read-only are **/proc/asound, /proc/bus, /proc/fs, /proc/irq, /proc/sys, /proc/sysrq-trigger, /sys/fs/cgroup**.

Note: Labeling can be disabled for all containers by setting label=false in the **containers.conf** (`/etc/containers/containers.conf` or `$HOME/.config/containers/containers.conf`) file.

#### **--shm-size**=*size*

Size of `/dev/shm` (format: `<number>[<unit>]`, where unit = b (bytes), k (kibibytes), m (mebibytes), or g (gibibytes))
If the unit is omitted, the system uses bytes. If the size is omitted, the system uses `64m`.
When size is `0`, there is no limit on the amount of memory used for IPC by the pod. This option conflicts with **--ipc=host** when running containers.

#### **--start**

When set to true, this flag starts the newly created pod after the
clone process has completed. All containers within the pod are started.

#### **--subgidname**=*name*

Name for GID map from the `/etc/subgid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--gidmap`.

#### **--subuidname**=*name*

Name for UID map from the `/etc/subuid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--uidmap`.

#### **--sysctl**=*name=value*

Configure namespace kernel parameters for all containers in the new pod.

For the IPC namespace, the following sysctls are allowed:

- kernel.msgmax
- kernel.msgmnb
- kernel.msgmni
- kernel.sem
- kernel.shmall
- kernel.shmmax
- kernel.shmmni
- kernel.shm_rmid_forced
- Sysctls beginning with fs.mqueue.\*

Note: if the ipc namespace is not shared within the pod, these sysctls are not allowed.

For the network namespace, only sysctls beginning with net.\* are allowed.

Note: if the network namespace is not shared within the pod, these sysctls are not allowed.

#### **--uidmap**=*container_uid:from_uid:amount*

Run all containers in the pod in a new user namespace using the supplied mapping. This
option conflicts with the **--userns** and **--subuidname** options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.

#### **--userns**=*mode*

Set the user namespace mode for all the containers in a pod. It defaults to the **PODMAN_USERNS** environment variable. An empty value ("") means user namespaces are disabled.

Rootless user --userns=Key mappings:

Key       | Host User |  Container User
----------|---------------|---------------------
""        |$UID           |0 (Default User account mapped to root user in container.)
keep-id   |$UID           |$UID (Map user account to same UID within container.)
auto      |$UID           | nil (Host User UID is not mapped into container.)
nomap     |$UID           | nil (Host User UID is not mapped into container.)

Valid _mode_ values are:

  - *auto[:*_OPTIONS,..._*]*: automatically create a namespace. It is possible to specify these options to `auto`:

  - *gidmapping=*_CONTAINER_GID:HOST_GID:SIZE_ to force a GID mapping to be present in the user namespace.

  - *size=*_SIZE_: to specify an explicit size for the automatic user namespace. e.g. `--userns=auto:size=8192`. If `size` is not specified, `auto` will estimate a size for the user namespace.

  - *uidmapping=*_CONTAINER_UID:HOST_UID:SIZE_ to force a UID mapping to be present in the user namespace.

  - *host*: run in the user namespace of the caller. The processes running in the container will have the same privileges on the host as any other process launched by the calling user (default).

  - *keep-id*: creates a user namespace where the current rootless user's UID:GID are mapped to the same values in the container. This option is ignored for containers created by the root user.

  - *nomap*: creates a user namespace where the current rootless user's UID:GID are not mapped into the container. This option is ignored for containers created by the root user.

#### **--uts**=*mode*

Set the UTS namespace mode for the pod. The following values are supported:

- **host**: use the host's UTS namespace inside the pod.
- **private**: create a new namespace for the pod (default).
- **ns:[path]**: run the pod in the given existing UTS namespace.


#### **--volume**, **-v**=*[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*

Create a bind mount. If ` -v /HOST-DIR:/CONTAINER-DIR` is specified, Podman
bind mounts `/HOST-DIR` in the host to `/CONTAINER-DIR` in the Podman
container. Similarly, `-v SOURCE-VOLUME:/CONTAINER-DIR` will mount the volume
in the host to the container. If no such named volume exists, Podman will
create one. The `OPTIONS` are a comma-separated list and can be: <sup>[[1]](#Footnote1)</sup>  (Note when using the remote client, including Mac and Windows (excluding WSL2) machines, the volumes will be mounted from the remote server, not necessarily the client machine.)

The _options_ is a comma-separated list and can be:

* **rw**|**ro**
* **z**|**Z**
* [**r**]**shared**|[**r**]**slave**|[**r**]**private**[**r**]**unbindable**
* [**r**]**bind**
* [**no**]**exec**
* [**no**]**dev**
* [**no**]**suid**
* [**O**]
* [**U**]

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The volume
will be mounted into the container at this directory.

Volumes may specify a source as well, as either a directory on the host
or the name of a named volume. If no source is given, the volume will be created as an
anonymously named volume with a randomly generated name, and will be removed when
the pod is removed via the `--rm` flag or `podman rm --volumes` commands.

If a volume source is specified, it must be a path on the host or the name of a
named volume. Host paths are allowed to be absolute or relative; relative paths
are resolved relative to the directory Podman is run in. If the source does not
exist, Podman will return an error. Users must pre-create the source files or
directories.

Any source that does not begin with a `.` or `/` will be treated as the name of
a named volume. If a volume with that name does not exist, it will be created.
Volumes created with names are not anonymous, and they are not removed by the `--rm`
option and the `podman rm --volumes` command.

Specify multiple  **-v** options to mount one or more volumes into a
pod.

  `Write Protected Volume Mounts`

Add `:ro` or `:rw` suffix to a volume to mount it read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

  `Chowning Volume Mounts`

By default, Podman does not change the owner and group of source volume
directories mounted into containers. If a pod is created in a new user
namespace, the UID and GID in the container may correspond to another UID and
GID on the host.

The `:U` suffix tells Podman to use the correct host UID and GID based on the
UID and GID within the pod, to change recursively the owner and group of
the source volume.

**Warning** use with caution since this will modify the host filesystem.

  `Labeling Volume Mounts`

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a pod. Without a label, the security system might
prevent the processes running inside the pod from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the pod context, add either of two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The `z` option tells Podman that two pods
share the volume content. As a result, Podman labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Podman to label the content with a private unshared label.
Only the current pod can use a private volume.

  `Overlay Volume Mounts`

   The `:O` flag tells Podman to mount the directory from the host as a
temporary storage using the `overlay file system`. The pod processes
can modify content within the mountpoint which is stored in the
container storage in a separate directory. In overlay terms, the source
directory will be the lower, and the container storage directory will be the
upper. Modifications to the mount point are destroyed when the pod
finishes executing, similar to a tmpfs mount point being unmounted.

  Subsequent executions of the container will see the original source directory
content, any changes from previous pod executions no longer exist.

  One use case of the overlay mount is sharing the package cache from the
host into the container to allow speeding up builds.

  Note:

     - The `O` flag conflicts with other options listed above.
Content mounted into the container is labeled with the private label.
       On SELinux systems, labels in the source directory must be readable
by the infra container label. Usually containers can read/execute `container_share_t`
and can read/write `container_file_t`. If unable to change the labels on a
source volume, SELinux container separation must be disabled for the infra container/pod
to work.
     - The source directory mounted into the pod with an overlay mount
should not be modified, it can cause unexpected failures. It is recommended
to not modify the directory until the container finishes running.

  `Mounts propagation`

By default bind mounted volumes are `private`. That means any mounts done
inside pod will not be visible on host and vice versa. One can change
this behavior by specifying a volume mount propagation property. Making a
volume `shared` mounts done under that volume inside pod will be
visible on host and vice versa. Making a volume `slave` enables only one
way mount propagation and that is mounts done on host under that volume
will be visible inside container but not the other way around. <sup>[[1]](#Footnote1)</sup>

To control mount propagation property of a volume one can use the [**r**]**shared**,
[**r**]**slave**, [**r**]**private** or the [**r**]**unbindable** propagation flag.
Propagation property can be specified only for bind mounted volumes and not for
internal volumes or named volumes. For mount propagation to work the source mount
point (the mount point where source dir is mounted on) has to have the right propagation
properties. For shared volumes, the source mount point has to be shared. And for
slave volumes, the source mount point has to be either shared or slave.
<sup>[[1]](#Footnote1)</sup>

To recursively mount a volume and all of its submounts into a
pod, use the `rbind` option. By default the bind option is
used, and submounts of the source directory will not be mounted into the
pod.

Mounting the volume with the `nosuid` options means that SUID applications on
the volume will not be able to change their privilege. By default volumes
are mounted with `nosuid`.

Mounting the volume with the noexec option means that no executables on the
volume will be able to executed within the pod.

Mounting the volume with the nodev option means that no devices on the volume
will be able to be used by processes within the pod. By default volumes
are mounted with `nodev`.

If the `<source-dir>` is a mount point, then "dev", "suid", and "exec" options are
ignored by the kernel.

Use `df <source-dir>` to figure out the source mount and then use
`findmnt -o TARGET,PROPAGATION <source-mount-dir>` to figure out propagation
properties of source mount. If `findmnt` utility is not available, then one
can look at the mount entry for the source mount point in `/proc/self/mountinfo`. Look
at `optional fields` and see if any propagation properties are specified.
`shared:X` means mount is `shared`, `master:X` means mount is `slave` and if
nothing is there that means mount is `private`. <sup>[[1]](#Footnote1)</sup>

To change propagation properties of a mount point use `mount` command. For
example, if one wants to bind mount source directory `/foo` one can do
`mount --bind /foo /foo` and `mount --make-private --make-shared /foo`. This
will convert /foo into a `shared` mount point. Alternatively one can directly
change propagation properties of source mount. Say `/` is source mount for
`/foo`, then use `mount --make-shared /` to convert `/` into a `shared` mount.

Note: if the user only has access rights via a group, accessing the volume
from inside a rootless pod will fail.

#### **--volumes-from**=*container[:options]]*

Mount volumes from the specified container(s). Used to share volumes between
containers and pods. The *options* is a comma-separated list with the following available elements:

* **rw**|**ro**
* **z**

Mounts already mounted volumes from a source container into another
pod. Must supply the source's container-id or container-name.
To share a volume, use the --volumes-from option when running
the target container. Volumes can be shared even if the source container
is not running.

By default, Podman mounts the volumes in the same mode (read-write or
read-only) as it is mounted in the source container.
This can be changed by adding a `ro` or `rw` _option_.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a pod. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the pod context, add `z` to the volume mount.
This suffix tells Podman to relabel file objects on the shared volumes. The `z`
option tells Podman that two entities share the volume content. As a result,
Podman labels the content with a shared content label. Shared volume labels allow
all containers to read/write content.

If the location of the volume from the source container overlaps with
data residing on a target pod, then the volume hides
that data on the target.


## EXAMPLES
```
# podman pod clone pod-name
6b2c73ff8a1982828c9ae2092954bcd59836a131960f7e05221af9df5939c584
```

```
# podman pod clone --name=cloned-pod
d0cf1f782e2ed67e8c0050ff92df865a039186237a4df24d7acba5b1fa8cc6e7
6b2c73ff8a1982828c9ae2092954bcd59836a131960f7e05221af9df5939c584
```

```
# podman pod clone --destroy --cpus=5 d0cf1f782e2ed67e8c0050ff92df865a039186237a4df24d7acba5b1fa8cc6e7
6b2c73ff8a1982828c9ae2092954bcd59836a131960f7e05221af9df5939c584
```

```
# podman pod clone 2d4d4fca7219b4437e0d74fcdc272c4f031426a6eacd207372691207079551de new_name
5a9b7851013d326aa4ac4565726765901b3ecc01fcbc0f237bc7fd95588a24f9
```
## SEE ALSO
**[podman-pod-create(1)](podman-pod-create.1.md)**

## HISTORY
May 2022, Originally written by Charlie Doern <cdoern@redhat.com>
