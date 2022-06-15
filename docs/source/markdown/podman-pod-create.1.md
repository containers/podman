% podman-pod-create(1)

## NAME
podman\-pod\-create - Create a new pod

## SYNOPSIS
**podman pod create** [*options*]

## DESCRIPTION

Creates an empty pod, or unit of multiple containers, and prepares it to have
containers added to it. The pod id is printed to STDOUT. You can then use
**podman create --pod `<pod_id|pod_name>` ...** to add containers to the pod, and
**podman pod start `<pod_id|pod_name>`** to start the pod.

## OPTIONS

#### **--add-host**=_host_:_ip_

Add a custom host-to-IP mapping (host:ip)

Add a line to /etc/hosts. The format is hostname:ip. The **--add-host**
option can be set multiple times.
The /etc/hosts file is shared between all containers in the pod.

#### **--cgroup-parent**=*path*

Path to cgroups under which the cgroup for the pod will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

#### **--cpus**=*amount*

Set the total number of CPUs delegated to the pod. Default is 0.000 which indicates that there is no limit on computation power.

#### **--cpuset-cpus**=*amount*

Limit the CPUs to support execution. First CPU is numbered 0. Unlike --cpus this is of type string and parsed as a list of numbers

Format is 0-3,0,1

Examples of the List Format:

0-4,9           # bits 0, 1, 2, 3, 4, and 9 set
0-2,7,12-14     # bits 0, 1, 2, 7, 12, 13, and 14 set

#### **--device**=_host-device_[**:**_container-device_][**:**_permissions_]

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

Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)

#### **--dns**=*ipaddr*

Set custom DNS servers in the /etc/resolv.conf file that will be shared between all containers in the pod. A special option, "none" is allowed which disables creation of /etc/resolv.conf for the pod.

#### **--dns-opt**=*option*

Set custom DNS options in the /etc/resolv.conf file that will be shared between all containers in the pod.

#### **--dns-search**=*domain*

Set custom DNS search domains in the /etc/resolv.conf file that will be shared between all containers in the pod.

#### **--gidmap**=*container_gid:host_gid:amount*

GID map for the user namespace. Using this flag will run the container with user namespace enabled. It conflicts with the `--userns` and `--subgidname` flags.

#### **--help**, **-h**

Print usage statement.

#### **--hostname**=name

Set a hostname to the pod

#### **--infra**

Create an infra container and associate it with the pod. An infra container is a lightweight container used to coordinate the shared kernel namespace of a pod. Default: true.

#### **--infra-command**=*command*

The command that will be run to start the infra container. Default: "/pause".

#### **--infra-conmon-pidfile**=*file*

Write the pid of the infra container's **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to manage Podman containers and pods.

#### **--infra-image**=*image*

The custom image that will be used for the infra container.  Unless specified, Podman builds a custom local image which does not require pulling down an image.

#### **--infra-name**=*name*

The name that will be used for the pod's infra container.

#### **--ip**=*ip*

Specify a static IP address for the pod, for example **10.88.64.128**.
This option can only be used if the pod is joined to only a single network - i.e., **--network=network-name** is used at most once -
and if the pod is not joining another container's network namespace via **--network=container:_id_**.
The address must be within the network's IP address pool (default **10.88.0.0/16**).

To specify multiple static IP addresses per pod, set multiple networks using the **--network** option with a static IP address specified for each using the `ip` mode for that option.

#### **--ip6**=*ipv6*

Specify a static IPv6 address for the pod, for example **fd46:db93:aa76:ac37::10**.
This option can only be used if the pod is joined to only a single network - i.e., **--network=network-name** is used at most once -
and if the pod is not joining another container's network namespace via **--network=container:_id_**.
The address must be within the network's IPv6 address pool.

To specify multiple static IPv6 addresses per pod, set multiple networks using the **--network** option with a static IPv6 address specified for each using the `ip6` mode for that option.

#### **--label**=*label*, **-l**

Add metadata to a pod (e.g., --label com.example.key=value).

#### **--label-file**=*label*

Read in a line delimited file of labels.

#### **--mac-address**=*address*

Pod network interface MAC address (e.g. 92:d0:c6:0a:29:33)
This option can only be used if the pod is joined to only a single network - i.e., **--network=_network-name_** is used at most once -
and if the pod is not joining another container's network namespace via **--network=container:_id_**.

Remember that the MAC address in an Ethernet network must be unique.
The IPv6 link-local address will be based on the device's MAC address
according to RFC4862.

To specify multiple static MAC addresses per pod, set multiple networks using the **--network** option with a static MAC address specified for each using the `mac` mode for that option.


#### **--name**=*name*, **-n**

Assign a name to the pod.

#### **--network**=*mode*, **--net**

Set the network mode for the pod. Invalid if using **--dns**, **--dns-opt**, or **--dns-search** with **--network** that is set to **none** or **container:**_id_.

Valid _mode_ values are:

- **bridge[:OPTIONS,...]**: Create a network stack on the default bridge. This is the default for rootful containers. It is possible to specify these additional options:
  - **alias=name**: Add network-scoped alias for the container.
  - **ip=IPv4**: Specify a static ipv4 address for this container.
  - **ip=IPv6**: Specify a static ipv6 address for this container.
  - **mac=MAC**: Specify a static mac address for this container.
  - **interface_name**: Specify a name for the created network interface inside the container.

  For example to set a static ipv4 address and a static mac address, use `--network bridge:ip=10.88.0.10,mac=44:33:22:11:00:99`.
- \<network name or ID\>[:OPTIONS,...]: Connect to a user-defined network; this is the network name or ID from a network created by **[podman network create](podman-network-create.1.md)**. Using the network name implies the bridge network mode. It is possible to specify the same options described under the bridge mode above. You can use the **--network** option multiple times to specify additional networks.
- **none**: Create a network namespace for the container but do not configure network interfaces for it, thus the container has no network connectivity.
- **container:**_id_: Reuse another container's network stack.
- **host**: Do not create a network namespace, the container will use the host's network. Note: The host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.
- **ns:**_path_: Path to a network namespace to join.
- **private**: Create a new namespace for the container. This will use the **bridge** mode for rootful containers and **slirp4netns** for rootless ones.
- **slirp4netns[:OPTIONS,...]**: use **slirp4netns**(1) to create a user network stack. This is the default for rootless containers. It is possible to specify these additional options, they can also be set with `network_cmd_options` in containers.conf:
  - **allow_host_loopback=true|false**: Allow the slirp4netns to reach the host loopback IP (`10.0.2.2`). Default is false.
  - **mtu=MTU**: Specify the MTU to use for this network. (Default is `65520`).
  - **cidr=CIDR**: Specify ip range to use for this network. (Default is `10.0.2.0/24`).
  - **enable_ipv6=true|false**: Enable IPv6. Default is true. (Required for `outbound_addr6`).
  - **outbound_addr=INTERFACE**: Specify the outbound interface slirp should bind to (ipv4 traffic only).
  - **outbound_addr=IPv4**: Specify the outbound ipv4 address slirp should bind to.
  - **outbound_addr6=INTERFACE**: Specify the outbound interface slirp should bind to (ipv6 traffic only).
  - **outbound_addr6=IPv6**: Specify the outbound ipv6 address slirp should bind to.
  - **port_handler=rootlesskit**: Use rootlesskit for port forwarding. Default.
  Note: Rootlesskit changes the source IP address of incoming packets to an IP address in the container network namespace, usually `10.0.2.100`. If your application requires the real source IP address, e.g. web server logs, use the slirp4netns port handler. The rootlesskit port handler is also used for rootless containers when connected to user-defined networks.
  - **port_handler=slirp4netns**: Use the slirp4netns port forwarding, it is slower than rootlesskit but preserves the correct source IP address. This port handler cannot be used for user-defined networks.

#### **--network-alias**=*alias*

Add a network-scoped alias for the pod, setting the alias for all networks that the pod joins. To set a name only for a specific network, use the alias option as described under the **--network** option.
Network aliases work only with the bridge networking mode. This option can be specified multiple times.
NOTE: A container will only have access to aliases on the first network that it joins. This is a limitation that will be removed in a later release.

#### **--no-hosts**

Do not create _/etc/hosts_ for the pod.
By default, Podman will manage _/etc/hosts_, adding the container's own IP address and any hosts from **--add-host**.
**--no-hosts** disables this, and the image's _/etc/hosts_ will be preserved unmodified.
This option conflicts with **--add-host**.

#### **--pid**=*pid*

Set the PID mode for the pod. The default is to create a private PID namespace for the pod. Requires the PID namespace to be shared via --share.

    host: use the host’s PID namespace for the pod
    ns: join the specified PID namespace
    private: create a new namespace for the pod (default)

#### **--pod-id-file**=*path*

Write the pod ID to the file.

#### **--publish**, **-p**=[[_ip_:][_hostPort_]:]_containerPort_[/_protocol_]

Publish a container's port, or range of ports, within this pod to the host.

Both hostPort and containerPort can be specified as a range of ports.
When specifying ranges for both, the number of container ports in the
range must match the number of host ports in the range.

If host IP is set to 0.0.0.0 or not set at all, the port will be bound on all IPs on the host.

By default, Podman will publish TCP ports. To publish a UDP port instead, give
`udp` as protocol. To publish both TCP and UDP ports, set `--publish` twice,
with `tcp`, and `udp` as protocols respectively. Rootful containers can also
publish ports using the `sctp` protocol.

Host port does not have to be specified (e.g. `podman run -p 127.0.0.1::80`).
If it is not, the container port will be randomly assigned a port on the host.

Use **podman port** to see the actual mapping: `podman port $CONTAINER $CONTAINERPORT`.

**Note:** You must not publish ports of containers in the pod individually,
but only by the pod itself.

**Note:** This cannot be modified once the pod is created.

#### **--replace**

If another pod with the same name already exists, replace and remove it.  The default is **false**.

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

- `no-new-privileges` : Disable container processes from gaining additional privileges

- `seccomp=unconfined` : Turn off seccomp confinement for the pod
- `seccomp=profile.json` :  Whitelisted syscalls seccomp Json file to be used as a seccomp filter

- `proc-opts=OPTIONS` : Comma-separated list of options to use for the /proc mount. More details for the
  possible mount options are specified in the **proc(5)** man page.

- **unmask**=_ALL_ or _/path/1:/path/2_, or shell expanded paths (/proc/*): Paths to unmask separated by a colon. If set to **ALL**, it will unmask all the paths that are masked or made read only by default.
  The default masked paths are **/proc/acpi, /proc/kcore, /proc/keys, /proc/latency_stats, /proc/sched_debug, /proc/scsi, /proc/timer_list, /proc/timer_stats, /sys/firmware, and /sys/fs/selinux.**  The default paths that are read only are **/proc/asound, /proc/bus, /proc/fs, /proc/irq, /proc/sys, /proc/sysrq-trigger, /sys/fs/cgroup**.

Note: Labeling can be disabled for all containers by setting label=false in the **containers.conf** (`/etc/containers/containers.conf` or `$HOME/.config/containers/containers.conf`) file.

#### **--share**=*namespace*

A comma-separated list of kernel namespaces to share. If none or "" is specified, no namespaces will be shared. The namespaces to choose from are cgroup, ipc, net, pid, uts.

The operator can identify a pod in three ways:
UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”)
UUID short identifier (“f78375b1c487”)
Name (“jonah”)

podman generates a UUID for each pod, and if a name is not assigned
to the container with **--name** then a random string name will be generated
for it. The name is useful any place you need to identify a pod.

#### **--share-parent**

This boolean determines whether or not all containers entering the pod will use the pod as their cgroup parent. The default value of this flag is true. If you are looking to share the cgroup namespace rather than a cgroup parent in a pod, use **--share**

Note: This options conflict with **--share=cgroup** since that would set the pod as the cgroup parent but enter the container into the same cgroupNS as the infra container.

#### **--subgidname**=*name*

Name for GID map from the `/etc/subgid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--gidmap`.

#### **--subuidname**=*name*

Name for UID map from the `/etc/subuid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--uidmap`.

#### **--sysctl**=_name_=_value_

Configure namespace kernel parameters for all containers in the pod.

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

#### **--uidmap**=*container_uid*:*from_uid*:*amount*

Run the container in a new user namespace using the supplied mapping. This
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

#### **--volume**, **-v**[=*[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*]

Create a bind mount. If you specify, ` -v /HOST-DIR:/CONTAINER-DIR`, Podman
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

You can specify multiple  **-v** options to mount one or more volumes into a
pod.

  `Write Protected Volume Mounts`

You can add `:ro` or `:rw` suffix to a volume to mount it read-only or
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

To change a label in the pod context, you can add either of two suffixes
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
and can read/write `container_file_t`. If you cannot change the labels on a
source volume, SELinux container separation must be disabled for the infra container/pod
to work.
     - The source directory mounted into the pod with an overlay mount
should not be modified, it can cause unexpected failures. It is recommended
that you do not modify the directory until the container finishes running.

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
For mount propagation to work the source mount point (the mount point where source dir
is mounted on) has to have the right propagation properties. For shared volumes, the
source mount point has to be shared. And for slave volumes, the source mount point
has to be either shared or slave. <sup>[[1]](#Footnote1)</sup>

If you want to recursively mount a volume and all of its submounts into a
pod, then you can use the `rbind` option. By default the bind option is
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

#### **--volumes-from**[=*CONTAINER*[:*OPTIONS*]]

Mount volumes from the specified container(s). Used to share volumes between
containers and pods. The *options* is a comma-separated list with the following available elements:

* **rw**|**ro**
* **z**

Mounts already mounted volumes from a source container into another
pod. You must supply the source's container-id or container-name.
To share a volume, use the --volumes-from option when running
the target container. You can share volumes even if the source container
is not running.

By default, Podman mounts the volumes in the same mode (read-write or
read-only) as it is mounted in the source container.
You can change this by adding a `ro` or `rw` _option_.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a pod. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the pod context, you can add `z` to the volume mount.
This suffix tells Podman to relabel file objects on the shared volumes. The `z`
option tells Podman that two entities share the volume content. As a result,
Podman labels the content with a shared content label. Shared volume labels allow
all containers to read/write content.

If the location of the volume from the source container overlaps with
data residing on a target pod, then the volume hides
that data on the target.


## EXAMPLES

```
$ podman pod create --name test

$ podman pod create --infra=false

$ podman pod create --infra-command /top

$ podman pod create --publish 8443:443

$ podman pod create --network slirp4netns:outbound_addr=127.0.0.1,allow_host_loopback=true

$ podman pod create --network slirp4netns:cidr=192.168.0.0/24

$ podman pod create --network net1:ip=10.89.1.5 --network net2:ip=10.89.10.10
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **containers.conf(1)**


## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
