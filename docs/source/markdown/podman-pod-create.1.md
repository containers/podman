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

Add a host to the /etc/hosts file shared between all containers in the pod.

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
can be used to specify device permissions It is a combination of
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if _host_device_ is a symbolic link then it will be resolved first.
The pod will only store the major and minor numbers of the host device.

Note: the pod implements devices by storing the initial configuration passed by the user and recreating the device on each container added to the pod.

Podman may load kernel modules required for using the specified
device. The devices that Podman will load modules for when necessary are:
/dev/fuse.

#### **--dns**=*ipaddr*

Set custom DNS servers in the /etc/resolv.conf file that will be shared between all containers in the pod. A special option, "none" is allowed which disables creation of /etc/resolv.conf for the pod.

#### **--dns-opt**=*option*

Set custom DNS options in the /etc/resolv.conf file that will be shared between all containers in the pod.

#### **--dns-search**=*domain*

Set custom DNS search domains in the /etc/resolv.conf file that will be shared between all containers in the pod.

#### **--gidmap**=*container_gid:host_gid:amount*

GID map for the user namespace. Using this flag will run the container with user namespace enabled. It conflicts with the `--userns` and `--subgidname` flags.

#### **--uidmap**=*container_uid*:*from_uid*:*amount*

Run the container in a new user namespace using the supplied mapping. This
option conflicts with the **--userns** and **--subuidname** options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.

#### **--subgidname**=*name*

Name for GID map from the `/etc/subgid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--gidmap`.

#### **--subuidname**=*name*

Name for UID map from the `/etc/subuid` file. Using this flag will run the container with user namespace enabled. This flag conflicts with `--userns` and `--uidmap`.

#### **--help**, **-h**

Print usage statement.

#### **--hostname**=name

Set a hostname to the pod

#### **--infra**

Create an infra container and associate it with the pod. An infra container is a lightweight container used to coordinate the shared kernel namespace of a pod. Default: true.

#### **--infra-conmon-pidfile**=*file*

Write the pid of the infra container's **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to manage Podman containers and pods.

#### **--infra-command**=*command*

The command that will be run to start the infra container. Default: "/pause".

#### **--infra-image**=*image*

The image that will be created for the infra container. Default: "k8s.gcr.io/pause:3.1".

#### **--infra-name**=*name*

The name that will be used for the pod's infra container.

#### **--ip**=*ipaddr*

Set a static IP for the pod's shared network.

#### **--label**=*label*, **-l**

Add metadata to a pod (e.g., --label com.example.key=value).

#### **--label-file**=*label*

Read in a line delimited file of labels.

#### **--mac-address**=*address*

Set a static MAC address for the pod's shared network.

#### **--name**=*name*, **-n**

Assign a name to the pod.

#### **--network**=*mode*, **--net**

Set network mode for the pod. Supported values are:
- **bridge**: Create a network stack on the default bridge. This is the default for rootfull containers.
- **none**: Create a network namespace for the container but do not configure network interfaces for it, thus the container has no network connectivity.
- **host**: Do not create a network namespace, all containers in the pod will use the host's network. Note: the host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.
- **network**: Connect to a user-defined network, multiple networks should be comma-separated.
- **private**: Create a new namespace for the container. This will use the **bridge** mode for rootfull containers and **slirp4netns** for rootless ones.
- **slirp4netns[:OPTIONS,...]**: use **slirp4netns**(1) to create a user network stack. This is the default for rootless containers. It is possible to specify these additional options:
  - **allow_host_loopback=true|false**: Allow the slirp4netns to reach the host loopback IP (`10.0.2.2`, which is added to `/etc/hosts` as `host.containers.internal` for your convenience). Default is false.
  - **mtu=MTU**: Specify the MTU to use for this network. (Default is `65520`).
  - **cidr=CIDR**: Specify ip range to use for this network. (Default is `10.0.2.0/24`).
  - **enable_ipv6=true|false**: Enable IPv6. Default is false. (Required for `outbound_addr6`).
  - **outbound_addr=INTERFACE**: Specify the outbound interface slirp should bind to (ipv4 traffic only).
  - **outbound_addr=IPv4**: Specify the outbound ipv4 address slirp should bind to.
  - **outbound_addr6=INTERFACE**: Specify the outbound interface slirp should bind to (ipv6 traffic only).
  - **outbound_addr6=IPv6**: Specify the outbound ipv6 address slirp should bind to.
  - **port_handler=rootlesskit**: Use rootlesskit for port forwarding. Default.
  Note: Rootlesskit changes the source IP address of incoming packets to a IP address in the container network namespace, usually `10.0.2.100`. If your application requires the real source IP address, e.g. web server logs, use the slirp4netns port handler. The rootlesskit port handler is also used for rootless containers when connected to user-defined networks.
  - **port_handler=slirp4netns**: Use the slirp4netns port forwarding, it is slower than rootlesskit but preserves the correct source IP address. This port handler cannot be used for user-defined networks.

#### **--network-alias**=strings

Add a DNS alias for the pod. When the pod is joined to a CNI network with support for the dnsname plugin, the containers inside the pod will be accessible through this name from other containers in the network.

#### **--no-hosts**

Disable creation of /etc/hosts for the pod.

#### **--pid**=*pid*

Set the PID mode for the pod. The default is to create a private PID namespace for the pod. Requires the PID namespace to be shared via --share.

    host: use the host’s PID namespace for the pod
    ns: join the specified PID namespace
    private: create a new namespace for the pod (default)

#### **--pod-id-file**=*path*

Write the pod ID to the file.

#### **--publish**=*port*, **-p**

Publish a port or range of ports from the pod to the host.

Format: `ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort`
Both hostPort and containerPort can be specified as a range of ports.
When specifying ranges for both, the number of container ports in the range must match the number of host ports in the range.
Use `podman port` to see the actual mapping: `podman port CONTAINER $CONTAINERPORT`.

NOTE: This cannot be modified once the pod is created.

#### **--replace**

If another pod with the same name already exists, replace and remove it.  The default is **false**.

#### **--share**=*namespace*

A comma-separated list of kernel namespaces to share. If none or "" is specified, no namespaces will be shared. The namespaces to choose from are ipc, net, pid, uts.

The operator can identify a pod in three ways:
UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”)
UUID short identifier (“f78375b1c487”)
Name (“jonah”)

podman generates a UUID for each pod, and if a name is not assigned
to the container with **--name** then a random string name will be generated
for it. The name is useful any place you need to identify a pod.

#### **--userns**=*mode*

Set the user namespace mode for all the containers in a pod. It defaults to the **PODMAN_USERNS** environment variable. An empty value ("") means user namespaces are disabled.

Valid _mode_ values are:

- *auto[:*_OPTIONS,..._*]*: automatically create a namespace. It is possible to specify these options to `auto`:
  - *gidmapping=*_CONTAINER_GID:HOST_GID:SIZE_ to force a GID mapping to be present in the user namespace.
  - *size=*_SIZE_: to specify an explicit size for the automatic user namespace. e.g. `--userns=auto:size=8192`. If `size` is not specified, `auto` will estimate a size for the user namespace.
  - *uidmapping=*_CONTAINER_UID:HOST_UID:SIZE_ to force a UID mapping to be present in the user namespace.
- *host*: run in the user namespace of the caller. The processes running in the container will have the same privileges on the host as any other process launched by the calling user (default).
- *keep-id*: creates a user namespace where the current rootless user's UID:GID are mapped to the same values in the container. This option is ignored for containers created by the root user.

#### **--volume**, **-v**[=*[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*]

Create a bind mount. If you specify, ` -v /HOST-DIR:/CONTAINER-DIR`, Podman
bind mounts `/HOST-DIR` in the host to `/CONTAINER-DIR` in the Podman
container. Similarly, `-v SOURCE-VOLUME:/CONTAINER-DIR` will mount the volume
in the host to the container. If no such named volume exists, Podman will
create one. The `OPTIONS` are a comma-separated list and can be: <sup>[[1]](#Footnote1)</sup>  (Note when using the remote client, the volumes will be mounted from the remote server, not necessarily the client machine.)

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
Propagation property can be specified only for bind mounted volumes and not for
internal volumes or named volumes. For mount propagation to work the source mount
point (the mount point where source dir is mounted on) has to have the right propagation
properties. For shared volumes, the source mount point has to be shared. And for
slave volumes, the source mount point has to be either shared or slave.
<sup>[[1]](#Footnote1)</sup>

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


## EXAMPLES

```
$ podman pod create --name test

$ podman pod create --infra=false

$ podman pod create --infra-command /top

$ podman pod create --publish 8443:443

$ podman pod create --network slirp4netns:outbound_addr=127.0.0.1,allow_host_loopback=true

$ podman pod create --network slirp4netns:cidr=192.168.0.0/24
```

## SEE ALSO
podman-pod(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
