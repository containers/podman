% podman-run(1)

## NAME
podman\-run - Run a command in a new container

## SYNOPSIS
**podman run** [*options*] *image* [*command* [*arg* ...]]

**podman container run** [*options*] *image* [*command* [*arg* ...]]

## DESCRIPTION

Run a process in a new container. **podman run** starts a process with its own
file system, its own networking, and its own isolated process tree. The _image_
which starts the process may define defaults related to the process that will be
run in the container, the networking to expose, and more, but **podman run**
gives final control to the operator or administrator who starts the container
from the image. For that reason **podman run** has more options than any other
podman command.

If the _image_ is not already loaded then **podman run** will pull the _image_, and
all image dependencies, from the repository in the same way running **podman
pull** _image_ , before it starts the container from that image.

Several files will be automatically created within the container. These include
_/etc/hosts_, _/etc/hostname_, and _/etc/resolv.conf_ to manage networking.
These will be based on the host's version of the files, though they can be
customized with options (for example, **--dns** will override the host's DNS
servers in the created _resolv.conf_). Additionally, an empty file is created in
each container to indicate to programs they are running in a container. This file
is located at _/run/.containerenv_.

When running from a user defined network namespace, the _/etc/netns/NSNAME/resolv.conf_
will be used if it exists, otherwise _/etc/resolv.conf_ will be used.

## OPTIONS
**--add-host**=_host_:_ip_

Add a line to container's _/etc/hosts_ for custom host-to-IP mapping.
This option can be set multiple times.

**--annotation**=_key_=_value_

Add an annotation to the container.
This option can be set multiple times.

**--attach**, **-a**=**stdin**|**stdout**|**stderr**

Attach to STDIN, STDOUT or STDERR.

In foreground mode (the default when **-d**
is not specified), **podman run** can start the process in the container
and attach the console to the process's standard input, output, and
error. It can even pretend to be a TTY (this is what most commandline
executables expect) and pass along signals. The **-a** option can be set for
each of **stdin**, **stdout**, and **stderr**.

**--authfile**[=*path*]

Path to the authentication file. Default is *${XDG_RUNTIME_DIR}/containers/auth.json*. (Not available for remote commands)

Note: You can also override the default path of the authentication file by setting the **REGISTRY_AUTH_FILE**
environment variable.

**--blkio-weight**=*weight*

Block IO relative weight. The _weight_ is a value between **10** and **1000**.

**--blkio-weight-device**=*device*:*weight*

Block IO relative device weight.

**--cap-add**=*capability*

Add Linux capabilities.

**--cap-drop**=*capability*

Drop Linux capabilities.

**--cgroupns**=*mode*

Set the cgroup namespace mode for the container.

- **host**: use the host's cgroup namespace inside the container.
- **container:**_id_: join the namespace of the specified container.
- **private**: create a new cgroup namespace.
- **ns:**_path_: join the namespace at the specified path.

If the host uses cgroups v1, the default is set to **host**.  On cgroups v2, the default is **private**.

**--cgroups**=**enabled**|**disabled**|**no-conmon**

Determines whether the container will create CGroups.

Default is **enabled**. The **disabled** option will force the container
to not create CGroups, and thus conflicts with CGroup options
(**--cgroupns** and **--cgroup-parent**).
The **no-conmon** option disables a new CGroup only for the **conmon** process.

**--cgroup-parent**=*path*

Path to cgroups under which the cgroup for the container will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

**--cidfile**=*file*

Write the container ID to *file*.

**--conmon-pidfile**=*file*

Write the pid of the **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to restart Podman containers.

**--cpu-period**=*limit*

Limit the container's CPU usage by setting CPU CFS (Completely Fair Scheduler) period.

**--cpu-quota**=*limit*

Limit the CPU CFS (Completely Fair Scheduler) quota.

Limit the container's CPU usage. By default, containers run with the full
CPU resource. This flag tell the kernel to restrict the container's CPU usage
to the quota you specify.

**--cpu-rt-period**=*microseconds*

Limit the CPU real-time period in microseconds.

Limit the container's Real Time CPU usage. This flag tell the kernel to restrict the container's Real Time CPU usage to the period you specify.

**--cpu-rt-runtime**=*microseconds*

Limit the CPU real-time runtime in microseconds.

Limit the containers Real Time CPU usage. This flag tells the kernel to limit the amount of time in a given CPU period Real Time tasks may consume. Ex:
Period of 1,000,000us and Runtime of 950,000us means that this container could consume 95% of available CPU and leave the remaining 5% to normal priority tasks.

The sum of all runtimes across containers cannot exceed the amount allotted to the parent cgroup.

**--cpu-shares**=*shares*

CPU shares (relative weight).

By default, all containers get the same proportion of CPU cycles. This proportion
can be modified by changing the container's CPU share weighting relative
to the combined weight of all the running containers. Default weight is **1024**.

The proportion will only apply when CPU-intensive processes are running.
When tasks in one container are idle, other containers can use the
left-over CPU time. The actual amount of CPU time will vary depending on
the number of containers running on the system.

For example, consider three containers, one has a cpu-share of 1024 and
two others have a cpu-share setting of 512. When processes in all three
containers attempt to use 100% of CPU, the first container would receive
50% of the total CPU time. If you add a fourth container with a cpu-share
of 1024, the first container only gets 33% of the CPU. The remaining containers
receive 16.5%, 16.5% and 33% of the CPU.

On a multi-core system, the shares of CPU time are distributed over all CPU
cores. Even if a container is limited to less than 100% of CPU time, it can
use 100% of each individual CPU core.

For example, consider a system with more than three cores. If you start one
container **{C0}** with **--cpu-shares=512** running one process, and another container
**{C1}** with **--cpu-shares=1024** running two processes, this can result in the following
division of CPU shares:

| PID  |  container  | CPU     | CPU share    |
| ---- | ----------- | ------- | ------------ |
| 100  |  {C0}       | 0       | 100% of CPU0 |
| 101  |  {C1}       | 1       | 100% of CPU1 |
| 102  |  {C1}       | 2       | 100% of CPU2 |

**--cpus**=*number*

Number of CPUs. The default is *0.0* which means no limit.

**--cpuset-cpus**=*number*

CPUs in which to allow execution. Can be specified as a comma-separated list
(e.g. **0,1**), as a range (e.g. **0-3**), or any combination thereof
(e.g. **0-3,7,11-15**).

**--cpuset-mems**=*nodes*

Memory nodes (MEMs) in which to allow execution. Only effective on NUMA systems.

For example, if you have four memory nodes (0-3) on your system, use **--cpuset-mems=0,1**
to only use memory from the first two memory nodes.

**--detach**, **-d**=**true**|**false**

Detached mode: run the container in the background and print the new container ID. The default is **false**.

At any time you can run **podman ps** in
the other shell to view a list of the running containers. You can reattach to a
detached container with **podman attach**.

When attached in the tty mode, you can detach from the container (and leave it
running) using a configurable key sequence.

**--detach-keys**=_sequence_

Specify the key sequence for detaching a container; _sequence_ is a comma-delimited set
in which each item can be a single character from the [a-Z] range,
or **ctrl**-_value_, where _value_ is one of: **a-z** or **@^[,_**.

This option can also be set in **libpod.conf**(5) file.

Specifying "" will disable this feature. The default is **ctrl-p,ctrl-q**.

**--device**=_host-device_[**:**_container-device_][**:**_permissions_]

Add a host device to the container. Optional *permissions* parameter
can be used to specify device permissions, it is combination of
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if _host_device_ is a symbolic link then it will be resolved first.
The container will only store the major and minor numbers of the host device.

Note: if the user only has access rights via a group, accessing the device
from inside a rootless container will fail. The **crun**(1) runtime offers a
workaround for this by adding the option **--annotation run.oci.keep_original_groups=1**.

**--device-cgroup-rule**=rule

Add a rule to the cgroup allowed devices list

**--device-read-bps**=_path_:_rate_

Limit read rate (in bytes per second) from a device (e.g. **--device-read-bps=/dev/sda:1mb**).

**--device-read-iops**=_path_:_rate_

Limit read rate (in IO operations per second) from a device (e.g. **--device-read-iops=/dev/sda:1000**).

**--device-write-bps**=_path_:_rate_

Limit write rate (in bytes per second) to a device (e.g. **--device-write-bps=/dev/sda:1mb**).

**--device-write-iops**=_path_:_rate_

Limit write rate (in IO operations per second) to a device (e.g. **--device-write-iops=/dev/sda:1000**).

**--dns**=*ipaddr*

Set custom DNS servers. Invalid if using **--dns** with **--network** that is set to **none** or **container:**_id_.

This option can be used to override the DNS
configuration passed to the container. Typically this is necessary when the
host DNS configuration is invalid for the container (e.g., **127.0.0.1**). When this
is the case the **--dns** flags is necessary for every run.

The special value **none** can be specified to disable creation of _/etc/resolv.conf_ in the container by Podman.
The _/etc/resolv.conf_ file in the image will be used without changes.

**--dns-opt**=*option*

Set custom DNS options. Invalid if using **--dns-opt** with **--network** that is set to **none** or **container:**_id_.

**--dns-search**=*domain*

Set custom DNS search domains. Invalid if using **--dns-search** and **--network** that is set to **none** or **container:**_id_.
Use **--dns-search=.** if you don't wish to set the search domain.

**--entrypoint**=*"command"* | *'["command", "arg1", ...]'*

Overwrite the default ENTRYPOINT of the image.

This option allows you to overwrite the default entrypoint of the image.

The ENTRYPOINT of an image is similar to a COMMAND
because it specifies what executable to run when the container starts, but it is
(purposely) more difficult to override. The ENTRYPOINT gives a container its
default nature or behavior, so that when you set an ENTRYPOINT you can run the
container as if it were that binary, complete with default options, and you can
pass in more options via the COMMAND. But, sometimes an operator may want to run
something else inside the container, so you can override the default ENTRYPOINT
at runtime by using a **--entrypoint** and a string to specify the new
ENTRYPOINT.

You need to specify multi option commands in the form of a json string.

**--env**, **-e**=*env*

Set environment variables.

This option allows arbitrary environment variables that are available for the process to be launched inside of the container.  If an environment variable is specified without a value, Podman will check the host environment for a value and set the variable only if it is set on the host.  If an environment variable ending in __*__ is specified, Podman will search the host environment for variables starting with the prefix and will add those variables to the container.  If an environment variable with a trailing ***** is specified, then a value must be supplied.

See [**Environment**](#environment) note below for precedence and examples.

**--env-host**=**true**|**false**

Use host environment inside of the container. See **Environment** note below for precedence. (Not available for remote commands)

**--env-file**=*file*

Read in a line delimited file of environment variables. See **Environment** note below for precedence.

**--expose**=*port*

Expose a port, or a range of ports (e.g. **--expose=3300-3310**) to set up port redirection
on the host system.

**--gidmap**=*container_gid*:*host_gid*:*amount*

Run the container in a new user namespace using the supplied mapping. This option conflicts with the **--userns** and **--subgidname** flags.
This option can be passed several times to map different ranges. If calling **podman run** as an unprivileged user, the user needs to have the right to use the mapping. See **subuid**(5).
The example maps gids **0-1999** in the container to the gids **30000-31999** on the host: **--gidmap=0:30000:2000**.

**--group-add**=*group*

Add additional groups to run as

**--health-cmd**=*"command"* | *'["command", "arg1", ...]'*

Set or alter a healthcheck command for a container. The command is a command to be executed inside your
container that determines your container health.  The command is required for other healthcheck options
to be applied.  A value of **none** disables existing healthchecks.

Multiple options can be passed in the form of a JSON array; otherwise, the command will be interpreted
as an argument to **/bin/sh -c**.

**--health-interval**=*interval*

Set an interval for the healthchecks. An _interval_ of **disable** results in no automatic timer setup. The default is **30s**.

**--health-retries**=*retries*

The number of retries allowed before a healthcheck is considered to be unhealthy. The default value is **3**.

**--health-start-period**=*period*

The initialization time needed for a container to bootstrap. The value can be expressed in time format like
**2m3s**.  The default value is **0s**.

**--health-timeout**=*timeout*

The maximum time allowed to complete the healthcheck before an interval is considered failed.  Like start-period, the
value can be expressed in a time format such as **1m22s**.  The default value is **30s**.

**--help**

Print usage statement

**-h**, **--hostname**=*name*

Container host name

Sets the container host name that is available inside the container.

**--http-proxy**=**true**|**false**

By default proxy environment variables are passed into the container if set
for the Podman process.  This can be disabled by setting the value to **false**.
The environment variables passed in include **http_proxy**,
**https_proxy**, **ftp_proxy**, **no_proxy**, and also the upper case versions of
those.  This option is only needed when the host system must use a proxy but
the container should not use any proxy.  Proxy environment variables specified
for the container in any other way will override the values that would have
been passed through from the host.  (Other ways to specify the proxy for the
container include passing the values with the **--env** flag, or hard coding the
proxy environment at container build time.) (Not available for remote commands)

Defaults to **true**.

**--image-volume**, **builtin-volume**=**bind**|**tmpfs**|**ignore**

Tells Podman how to handle the builtin image volumes. Default is **bind**.

- **bind**: An anonymous named volume will be created and mounted into the container.
- **tmpfs**: The volume is mounted onto the container as a tmpfs, which allows the users to create
content that disappears when the container is stopped.
- **ignore**: All volumes are just ignored and no action is taken.

**--init**

Run an init inside the container that forwards signals and reaps processes.

**--init-path**=*path*

Path to the container-init binary.

**--interactive**, **-i**=**true**|**false**

When set to **true**, keep stdin open even if not attached. The default is **false**.

**--ip6**=*ip*

Not implemented.

**--ip**=*ip*

Specify a static IP address for the container, for example **10.88.64.128**.
Can only be used if no additional CNI networks to join were specified via **--network=**_network-name_,
and if the container is not joining another container's network namespace via **--network=container:**_id_.
The address must be within the default CNI network's pool (default **10.88.0.0/16**).

**--ipc**=*mode*

Set the IPC namespace mode for a container. The default is to create
a private IPC namespace.

- **container:**_id_: reuses another container shared memory, semaphores and message queues
- **host**: use the host shared memory,semaphores and message queues inside the container.  Note: the host mode gives the container full access to local shared memory and is therefore considered insecure.
- **ns:**_path_: path to an IPC namespace to join.

**--kernel-memory**=_number_[_unit_]

Kernel memory limit. A _unit_ can be **b** (bytes), **k** (kilobytes), **m** (megabytes), or **g** (gigabytes).

Constrains the kernel memory available to a container. If a limit of 0
is specified (not using *--kernel-memory*), the container's kernel memory
is not limited. If you specify a limit, it may be rounded up to a multiple
of the operating system's page size and the value can be very large,
millions of trillions.

**--label**, **-l**=*key*=*value*

Add metadata to a container.

**--label-file**=*file*

Read in a line-delimited file of labels.

**--link-local-ip**=*ip*

Not implemented.

**--log-driver**="*driver*"

Logging driver for the container. Currently available options are **k8s-file** and **journald**, with **json-file** aliased to **k8s-file** for scripting compatibility.

**--log-opt**=*name*=*value*

Set custom logging configuration. The following *name*s are supported:
- **path**: specify a path to the log file (e.g.
**--log-opt path=/var/log/container/mycontainer.json**);
- **tag**: specify a custom log tag for the container (e.g.
**--log-opt tag="{{.ImageName}}"**. It supports the same
keys as **podman inspect --format**.

This option is currently supported only by the **journald** log driver.

**--mac-address**=*address*

Container MAC address (e.g. **92:d0:c6:0a:29:33**).

Remember that the MAC address in an Ethernet network must be unique.
The IPv6 link-local address will be based on the device's MAC address
according to RFC4862.

**--memory**, **-m**=_number_[_unit_]

Memory limit. A _unit_ can be **b** (bytes), **k** (kilobytes), **m** (megabytes), or **g** (gigabytes).

Allows you to constrain the memory available to a container. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

**--memory-reservation**=_number_[_unit_]

Memory soft limit. A _unit_ can be **b** (bytes), **k** (kilobytes), **m** (megabytes), or **g** (gigabytes).

After setting memory reservation, when the system detects memory contention
or low memory, containers are forced to restrict their consumption to their
reservation. So you should always set the value below **--memory**, otherwise the
hard limit will take precedence. By default, memory reservation will be the same
as memory limit.

**--memory-swap**=_number_[_unit_]

A limit value equal to memory plus swap.
A _unit_ can be **b** (bytes), **k** (kilobytes), **m** (megabytes), or **g** (gigabytes).

Must be used with the  **-m** (**--memory**) flag.
The argument value should always be larger than that of
 **-m** (**--memory**).  By default, it is set to double
the value of **--memory**.

Set _number_ to **-1** to enable unlimited swap.

**--memory-swappiness**=*number*

Tune a container's memory swappiness behavior. Accepts an integer between *0* and *100*.

**--mount**=*type=TYPE,TYPE-SPECIFIC-OPTION[,...]*

Attach a filesystem mount to the container

Current supported mount TYPEs are **bind**, **volume**, and **tmpfs**.

       e.g.

       type=bind,source=/path/on/host,destination=/path/in/container

       type=bind,src=/path/on/host,dst=/path/in/container,relabel=shared

       type=volume,source=vol1,destination=/path/in/container,ro=true

       type=tmpfs,tmpfs-size=512M,destination=/path/in/container

       Common Options:

	      · src, source: mount source spec for bind and volume. Mandatory for bind.

	      · dst, destination, target: mount destination spec.

	      · ro, readonly: true or false (default).

       Options specific to bind:

	      · bind-propagation: shared, slave, private, rshared, rslave, or rprivate(default). See also mount(2).

	      . bind-nonrecursive: do not setup a recursive bind mount.  By default it is recursive.

	      . relabel: shared, private.

       Options specific to tmpfs:

	      · tmpfs-size: Size of the tmpfs mount in bytes. Unlimited by default in Linux.

	      · tmpfs-mode: File mode of the tmpfs in octal. (e.g. 700 or 0700.) Defaults to 1777 in Linux.

	      · tmpcopyup: Enable copyup from the image directory at the same location to the tmpfs.  Used by default.

	      · notmpcopyup: Disable copying files from the image to the tmpfs.

**--name**=*name*

Assign a name to the container.

The operator can identify a container in three ways:

- UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”);
- UUID short identifier (“f78375b1c487”);
- Name (“jonah”).

Podman generates a UUID for each container, and if a name is not assigned
to the container with **--name** then it will generate a random
string name. The name is useful any place you need to identify a container.
This works for both background and foreground containers.

**--network**, **--net**=*mode*

Set the network mode for the container. Invalid if using **--dns**, **--dns-opt**, or **--dns-search** with **--network** that is set to **none** or **container:**_id_.

Valid _mode_ values are:

- **bridge**: create a network stack on the default bridge;
- **none**: no networking;
- **container:**_id_: reuse another container's network stack;
- **host**: use the Podman host network stack. Note: the host mode gives the container full access to local system services such as D-bus and is therefore considered insecure;
- _network-id_: connect to a user-defined network, multiple networks should be comma separated;
- **ns:**_path_: path to a network namespace to join;
- `private`: create a new namespace for the container (default)
- **slirp4netns**: use **slirp4netns**(1) to create a user network stack.  This is the default for rootless containers.

**--network-alias**=*alias*

Not implemented.

**--no-healthcheck**=*true|false*

Disable any defined healthchecks for container.

**--no-hosts**=**true**|**false**

Do not create _/etc/hosts_ for the container.

By default, Podman will manage _/etc/hosts_, adding the container's own IP address and any hosts from **--add-host**.
**--no-hosts** disables this, and the image's _/etc/hosts_ will be preserved unmodified.
This option conflicts with **--add-host**.

**--oom-kill-disable**=**true**|**false**

Whether to disable OOM Killer for the container or not.

**--oom-score-adj**=*num*

Tune the host's OOM preferences for containers (accepts values from **-1000** to **1000**).

**--pid**=*mode*

Set the PID namespace mode for the container.
The efault is to create a private PID namespace for the container.

- **container:**_id_: join another container's PID namespace;
- **host**: use the host's PID namespace for the container. Note the host mode gives the container full access to local PID and is therefore considered insecure;
- **private**: create a new namespace for the container (default)
- **ns:**_path_: join the specified PID namespace.

**--pids-limit**=*limit*

Tune the container's pids limit. Set to **0** to have unlimited pids for the container. The default is **4096** on systems that support "pids" cgroup controller.

**--pod**=*name*

Run container in an existing pod. If you want Podman to make the pod for you, prefix the pod name with **new:**.
To make a pod with more granular options, use the **podman pod create** command before creating a container.
If a container is run with a pod, and the pod has an infra-container, the infra-container will be started before the container is.

**--privileged**=**true**|**false**

Give extended privileges to this container. The default is **false**.

By default, Podman containers are unprivileged (**=false**) and cannot, for
example, modify parts of the operating system.  This is because by default a
container is only allowed limited access to devices.  A "privileged" container
is given the same access to devices as the user launching the container.

A privileged container turns off the security features that isolate the
container from the host. Dropped Capabilities, limited devices, read-only mount
points, Apparmor/SELinux separation, and Seccomp filters are all disabled.

Rootless containers cannot have more privileges than the account that launched them.

**--publish**, **-p**=_ip_:_hostPort_:_containerPort_ | _ip_::_containerPort_ | _hostPort_:_containerPort_ | _containerPort_

Publish a container's port, or range of ports, to the host.

Both hostPort and containerPort can be specified as a range of ports.

When specifying ranges for both, the number of container ports in the range must match the number of host ports in the range.

Use **podman port** to see the actual mapping: **podman port $CONTAINER $CONTAINERPORT**.

**--publish-all**, **-P**=**true**|**false**

Publish all exposed ports to random ports on the host interfaces. The default is **false**.

When set to **true**, publish all exposed ports to the host interfaces. The
default is **false**. If the operator uses **-P** (or **-p**) then Podman will make the
exposed port accessible on the host and the ports will be available to any
client that can reach the host.

When using this option, Podman will bind any exposed port to a random port on the host
within an ephemeral port range defined by */proc/sys/net/ipv4/ip_local_port_range*.
To find the mapping between the host ports and the exposed ports, use **podman port**.

**--pull**=**always**|**missing**|**never**

Pull image before running. The default is **missing**.

- **missing**: attempt to pull the latest image from the registries listed in registries.conf if a local image does not exist.Raise an error if the image is not in any listed registry and is not present locally.
- **always**: Pull the image from the first registry it is found in as listed in  registries.conf. Raise an error if not found in the registries, even if the image is present locally.
- **never**: do not pull the image from the registry, use only the local version. Raise an error if the image is not present locally.

**--quiet**, **-q**

Suppress output information when pulling images

**--read-only**=**true**|**false**

Mount the container's root filesystem as read only.

By default a container will have its root filesystem writable allowing processes
to write files anywhere.  By specifying the **--read-only** flag, the container will have
its root filesystem mounted as read only prohibiting any writes.

**--read-only-tmpfs**=**true**|**false**

If container is running in **--read-only** mode, then mount a read-write tmpfs on _/run_, _/tmp_, and _/var/tmp_. The default is **true**.

**--restart**=*policy*

Restart policy to follow when containers exit.
Restart policy will not take effect if a container is stopped via the **podman kill** or **podman stop** commands.

Valid _policy_ values are:

- **no**: Do not restart containers on exit;
- **on-failure**[:*max_retries*]: Restart containers when they exit
with a non-zero exit code, retrying indefinitely or until the optional
*max_retries* count is hit;
- **always**: Restart containers when they exit, regardless of status, retrying indefinitely.

Please note that restart will not restart containers after a system reboot.
If this functionality is required in your environment, you can invoke Podman from a **systemd.unit**(5) file, or create an init script for whichever init system is in use.
To generate systemd unit files, please see **podman generate systemd**.

**--rm**=**true**|**false**

Automatically remove the container when it exits. The default is **false**.

Note that the container will not be removed when it could not be created or
started successfully. This allows the user to inspect the container after
failure.

**--rmi**=*true|false*

After exit of the container, remove the image unless another
container is using it. The default is *false*.

**--rootfs**

If specified, the first argument refers to an exploded container on the file system.

This is useful to run a container without requiring any image management, the rootfs
of the container is assumed to be managed externally.

Note: On **SELinux** systems, the rootfs needs the correct label, which is by default
**unconfined_u:object_r:container_file_t**.

**--seccomp-policy**=*policy*

Specify the policy to select the seccomp profile. If set to *image*, Podman will look for a "io.podman.seccomp.profile" label in the container-image config and use its value as a seccomp profile. Otherwise, Podman will follow the *default* policy by applying the default profile unless specified otherwise via *--security-opt seccomp* as described below.

Note that this feature is experimental and may change in the future.

**--security-opt**=*option*

Security Options

- **apparmor=unconfined** : Turn off apparmor confinement for the container
- **apparmor**=_your-profile_ : Set the apparmor confinement profile for the container
- **label=user:**_USER_: Set the label user for the container processes
- **label=role:**_ROLE_: Set the label role for the container processes
- **label=type:**_TYPE_: Set the label process type for the container processes
- **label=level:**_LEVEL_: Set the label level for the container processes
- **label=filetype:**TYPE_: Set the label file type for the container files
- **label=disable**: Turn off label separation for the container
- **no-new-privileges**: Disable container processes from gaining additional privileges
- **seccomp=unconfined**: Turn off seccomp confinement for the container
- **seccomp**=_profile.json_:  Whitelisted syscalls seccomp JSON file to be used as a seccomp filter

Note: Labeling can be disabled for all containers by setting **label=false** in the **libpod.conf**(5) file.

**--shm-size**=_number_[_unit_]

Size of _/dev/shm_. A _unit_  can be **b** (bytes), **k** (kilobytes), **m** (megabytes), or **g** (gigabytes).
If you omit the unit, the system uses bytes. If you omit the size entirely, the default is **64m**.
When _size_ is **0**, there is no limit on the amount of memory used for IPC by the container.

**--sig-proxy**=**true**|**false**

Sets whether the signals sent to the **podman run** command are proxied to the container process. SIGCHLD, SIGSTOP, and SIGKILL are not proxied. The default is **true**.

**--stop-signal**=*signal*

Signal to stop a container. Default is **SIGTERM**.

**--stop-timeout**=*seconds*

Timeout to stop a container. Default is **10**.

**--subgidname**=*name*

Run the container in a new user namespace using the map with _name_ in the _/etc/subgid_ file.
If calling **podman run** as an unprivileged user, the user needs to have the right to use the mapping. See **subgid**(5).
This flag conflicts with **--userns** and **--gidmap**.

**--subuidname**=*name*

Run the container in a new user namespace using the map with _name_ in the _/etc/subuid_ file.
If calling **podman run** as an unprivileged user, the user needs to have the right to use the mapping. See **subuid**(5).
This flag conflicts with **--userns** and **--uidmap**.

**--sysctl**=_name_=_value_

Configure namespaced kernel parameters at runtime.

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

Note: if you use the **--ipc=host** option, the above sysctls will not be allowed.

For the network namespace, the following ysctls areallowed:

- Sysctls beginning with net.\*

Note: if you use the **--network=host** option, these sysctls will not be allowed.

**--systemd**=**true**|**false**|**always**

Run container in systemd mode. The default is **true**.

The value *always* enforces the systemd mode is enforced without
looking at the executable name.  Otherwise, if set to **true** and the
command you are running inside the container is systemd, _/usr/sbin/init_
or _/sbin/init_.

If the command you are running inside of the container is systemd
Podman will setup tmpfs mount points in the following directories:

- _/run_
- _/run/lock_
- _/tmp_
- _/sys/fs/cgroup/systemd_
- _/var/lib/journal_

It will also set the default stop signal to **SIGRTMIN+3**.

This allows systemd to run in a confined container without any modifications.

Note that on **SELinux** systems, systemd attempts to write to the cgroup
file system.  Containers writing to the cgroup file system are denied by default.
The **container_manage_cgroup** boolean must be enabled for this to be allowed on an SELinux separated system.
```
setsebool -P container_manage_cgroup true
```

**--tmpfs**=*fs*

Create a tmpfs mount.

Mount a temporary filesystem (**tmpfs**) mount into a container, for example:

```
$ podman run -d --tmpfs /tmp:rw,size=787448k,mode=1777 my_image
```

This command mounts a **tmpfs** at _/tmp_ within the container.  The supported mount
options are the same as the Linux default mount flags. If you do not specify
any options, the systems uses the following options:
**rw,noexec,nosuid,nodev**.

**--tty**, **-t**=**true**|**false**

Allocate a pseudo-TTY. The default is **false**.

When set to **true**, Podman will allocate a pseudo-tty and attach to the standard
input of the container. This can be used, for example, to run a throwaway
interactive shell. The default is **false**.

**NOTE**: The **-t** option is incompatible with a redirection of the Podman client
standard input.

**--uidmap**=*container_uid*:*host_uid*:*amount*

Run the container in a new user namespace using the supplied mapping. This option conflicts
with the **--userns** and **--subuidname** flags.
This option can be passed several times to map different ranges. If calling **podman run**
as an unprivileged user, the user needs to have the right to use the mapping. See **subuid**(5).

The following example maps uids 0-1999 in the container to the uids 30000-31999 on the host: **--uidmap=0:30000:2000**.

**--ulimit**=*option*

Ulimit options. You can use **host** to copy the current configuration from the host.

**--user**, **-u**=[_user_ | _user_:_group_ | _uid_ | _uid_:_gid_ | _user_:_gid_ | _uid_:_group_ ]

Sets the username or UID used and optionally the groupname or GID for the specified command.

Without this argument the command will be run as root in the container.

**--userns**=**auto**|**host**|**keep-id**|**container:**_id_|**ns:**_namespace_

Set the user namespace mode for the container.  It defaults to the **PODMAN_USERNS** environment variable.  An empty value means user namespaces are disabled.

- **auto**: automatically create a namespace.  It is possible to specify other options to `auto`.  The supported options are
  **size=SIZE** to specify an explicit size for the automatic user namespace.  e.g. `--userns=auto:size=8192`.  If `size` is not specified, `auto` will guess a size for the user namespace.
  **uidmapping=HOST_UID:CONTAINER_UID:SIZE** to force a UID mapping to be present in the user namespace.
  **gidmapping=HOST_UID:CONTAINER_UID:SIZE** to force a GID mapping to be present in the user namespace.
- **host**: run in the user namespace of the caller. This is the default if no user namespace options are set. The processes running in the container will have the same privileges on the host as any other process launched by the calling user.
- **keep-id**: creates a user namespace where the current rootless user's UID:GID are mapped to the same values in the container. This option is ignored for containers created by the root user.
- **ns**: run the container in the given existing user namespace.
- **private**: create a new namespace for the container (default)
- **container**: join the user namespace of the specified container.

This option is incompatible with **--gidmap**, **--uidmap**, **--subuid** and **--subgid**.

**--uts**=*mode*

Set the UTS namespace mode for the container. The following values are supported:

- **host**: use the host's UTS namespace inside the container.
- **private**: create a new namespace for the container (default)
- **ns**: use own UTS namespace.

**NOTE**: the host mode gives the container access to changing the host's hostname and is therefore considered insecure.

**--volume**, **-v**[=[[_source-volume_|_host-dir_:]_container-dir_[:_options_]]]

Create a bind mount. If you specify _/host-dir_:_/container-dir_, Podman
bind mounts _host-dir_ in the host to _container-dir_ in the Podman
container. Similarly, _source-volume_:_/container-dir_ will mount the volume
in the host to the container. If no such named volume exists, Podman will
create one.

The _options_ is a comma delimited list and can be:

* **rw**|**ro**
* **z**|**Z**
* [**r**]**shared**|[**r**]**slave**|[**r**]**private**
* [**r**]**bind**
* [**no**]**exec**
* [**no**]**dev**
* [**no**]**suid**

The _container-dir_ must be an absolute path.

Volumes may specify a source as well, as either a directory on the host or the
name of a named volume. If no source is given, the volume will be created as an
anonymous named volume with a randomly generated name, and will be removed when
the container is removed via the **--rm** flag or **podman rm --volumes**.

If a volume source is specified, it must be a path on the host or the name of a
named volume. Host paths are allowed to be absolute or relative; relative paths
are resolved relative to the directory Podman is run in. Any source that does
not begin with a **.** or **/** it will be treated as the name of a named volume.
If a volume with that name does not exist, it will be created. Volumes created
with names are not anonymous and are not removed by **--rm** and
**podman rm --volumes**.

You can specify multiple  **-v** options to mount one or more volumes into a
container.

You can add **:ro** or **:rw** option to mount a volume in read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the container context, you can add either of two suffixes
**:z** or **:Z** to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The **z** option tells Podman that two containers
share the volume content. As a result, Podman labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The **Z** option tells Podman to label the content with a private unshared label.
Only the current container can use a private volume.

By default bind mounted volumes are **private**. That means any mounts done
inside container will not be visible on host and vice versa. One can change
this behavior by specifying a volume mount propagation property. Making a
volume shared mounts done under that volume inside container will be
visible on host and vice versa. Making a volume **slave** enables only one
way mount propagation and that is mounts done on host under that volume
will be visible inside container but not the other way around.

To control mount propagation property of volume one can use [**r**]**shared**,
[**r**]**slave** or [**r**]**private** propagation flag. Propagation property can
be specified only for bind mounted volumes and not for internal volumes or
named volumes. For mount propagation to work source mount point (mount point
where source dir is mounted on) has to have right propagation properties. For
shared volumes, source mount point has to be shared. And for slave volumes,
source mount has to be either shared or slave.

If you want to recursively mount a volume and all of its submounts into a
container, then you can use the **rbind** option.  By default the bind option is
used, and submounts of the source directory will not be mounted into the
container.

Mounting the volume with the **nosuid** options means that SUID applications on
the volume will not be able to change their privilege. By default volumes
are mounted with **nosuid**.

Mounting the volume with the **noexec** option means that no executables on the
volume will be able to executed within the container.

Mounting the volume with the **nodev** option means that no devices on the volume
will be able to be used by processes within the container. By default volumes
are mounted with **nodev**.

If the _host-dir_ is a mount point, then **dev**, **suid**, and **exec** options are
ignored by the kernel.

Use **df $hostdir** to figure out the source mount, and then use
**findmnt -o TARGET,PROPAGATION _source-mount-dir_** to figure out propagation
properties of source mount. If **findmnt**(1) utility is not available, then one
can look at mount entry for source mount point in _/proc/self/mountinfo_. Look
at the "optional fields" and see if any propagation properties are specified.
In there, **shared:N** means the mount is shared, **master:N** means mount
is slave, and if nothing is there, the mount is private.

To change propagation properties of a mount point, use **mount**(8) command. For
example, if one wants to bind mount source directory _/foo_, one can do
**mount --bind /foo /foo** and **mount --make-private --make-shared /foo**. This
will convert /foo into a shared mount point. Alternatively, one can directly
change propagation properties of source mount. Say, if _/_ is source mount for
_/foo_, then use **mount --make-shared /** to convert _/_ into a shared mount.

**--volumes-from**[=*container-id*[:*options*]]

Mount volumes from the specified container(s). Used to share volumes between
containers. The *options* is a comma delimited list with the following available elements:

* **rw**|**ro**
* **z**

You can share volumes even if the source container is not running.

By default, Podman mounts the volumes in the same mode (read-write or
read-only) as it is mounted in the source container.
You can change this by adding a **ro** or **rw** _option_.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the container context, you can add **z** to the volume mount.
This suffix tells Podman to relabel file objects on the shared volumes. The **z**
option tells Podman that two containers share the volume content. As a result,
podman labels the content with a shared content label. Shared volume labels allow
all containers to read/write content.

If the location of the volume from the source container overlaps with
data residing on a target container, then the volume hides
that data on the target.

**--workdir**, **-w**=*dir*

Working directory inside the container.

The default working directory for running binaries within a container is the root directory (**/**).
The image developer can set a different default with the WORKDIR instruction. The operator
can override the working directory by using the **-w** option.

## Exit Status

The exit code from **podman run** gives information about why the container
failed to run or why it exited.  When **podman run** exits with a non-zero code,
the exit codes follow the **chroot**(1) standard, see below:

  **125** The error is with Podman itself

    $ podman run --foo busybox; echo $?
    Error: unknown flag: --foo
    125

  **126** The _contained command_ cannot be invoked

    $ podman run busybox /etc; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"/etc\": permission denied": OCI runtime error
    126

  **127** The _contained command_ cannot be found

    $ podman run busybox foo; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"foo\": executable file not found in $PATH": OCI runtime error
    127

  **Exit code** _contained command_ exit code

    $ podman run busybox /bin/sh -c 'exit 3'
    3

## EXAMPLES

### Running container in read-only mode

During container image development, containers often need to write to the image
content.  Installing packages into _/usr_, for example.  In production,
applications seldom need to write to the image.  Container applications write
to volumes if they need to write to file systems at all.  Applications can be
made more secure by running them in read-only mode using the **--read-only** switch.
This protects the containers image from modification. Read only containers may
still need to write temporary data.  The best way to handle this is to mount
tmpfs directories on _/run_ and _/tmp_.

```
$ podman run --read-only -i -t fedora /bin/bash

$ podman run --read-only --read-only-tmpfs=false --tmpfs /run -i -t fedora /bin/bash
```

### Exposing log messages from the container to the host's log

If you want messages that are logged in your container to show up in the host's
syslog/journal then you should bind mount the _/dev/log_ directory as follows.

```
$ podman run -v /dev/log:/dev/log -i -t fedora /bin/bash
```

From inside the container you can test this by sending a message to the log.

```
(bash)# logger "Hello from my container"
```

Then exit and check the journal.

```
(bash)# exit

$ journalctl -b | grep Hello
```

This should list the message sent to logger.

### Attaching to one or more from STDIN, STDOUT, STDERR

If you do not specify **-a**, Podman will attach everything (stdin, stdout, stderr).
You can specify to which of the three standard streams (stdin, stdout, stderr)
you'd like to connect instead, as in:

```
$ podman run -a stdin -a stdout -i -t fedora /bin/bash
```

## Sharing IPC between containers

Using **shm_server.c** available here: https://www.cs.cf.ac.uk/Dave/C/node27.html

Testing **--ipc=host** mode:

Host shows a shared memory segment with 7 pids attached, happens to be from httpd:

```
$ sudo ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
0x01128e25 0          root       600        1000       7
```

Now run a regular container, and it correctly does NOT see the shared memory segment from the host:

```
$ podman run -it shm ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
```

Run a container with the new **--ipc=host** option, and it now sees the shared memory segment from the host httpd:

```
$ podman run -it --ipc=host shm ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
0x01128e25 0          root       600        1000       7
```
Testing **--ipc=container:**_id_ mode:

Start a container with a program to create a shared memory segment:
```
$ podman run -it shm bash
$ sudo shm/shm_server &
$ sudo ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
0x0000162e 0          root       666        27         1
```
Create a 2nd container correctly shows no shared memory segment from 1st container:
```
$ podman run shm ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
```

Create a 3rd container using the **--ipc=container:**_id_ option, now it shows the shared memory segment from the first:

```
$ podman run -it --ipc=container:ed735b2264ac shm ipcs -m
$ sudo ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
0x0000162e 0          root       666        27         1
```

### Mapping Ports for External Usage

The exposed port of an application can be mapped to a host port using the **-p**
flag. For example, an httpd port 80 can be mapped to the host port 8080 using the
following:

```
$ podman run -p 8080:80 -d -i -t fedora/httpd
```

### Mounting External Volumes

To mount a host directory as a container volume, specify the absolute path to
the directory and the absolute path for the container directory separated by a
colon. If the source is a named volume maintained by Podman, it's recommended to
use it's name rather than the path to the volume. Otherwise the volume will be
considered as an orphan and wiped if you execute **podman volume prune**:

```
$ podman run -v /var/db:/data1 -i -t fedora bash

$ podman run -v data:/data2 -i -t fedora bash
```

Using **--mount** flags to mount a host directory as a container folder, specify
the absolute path to the directory or the volume name, and the absolute path
within the container directory:

````
$ podman run --mount type=bind,src=/var/db,target=/data1 busybox sh

$ podman run --mount type=bind,src=volume-name,target=/data1 busybox sh
````

When using SELinux, be aware that the host has no knowledge of container SELinux
policy. Therefore, in the above example, if SELinux policy is enforced, the
_/var/db_ directory is not writable to the container. A "Permission Denied"
message will occur and an **avc:** message in the host's syslog.

To work around this, at time of writing this man page, the following command
needs to be run in order for the proper SELinux policy type label to be attached
to the host directory:

```
$ chcon -Rt svirt_sandbox_file_t /var/db
```

Now, writing to the _/data1_ volume in the container will be allowed and the
changes will also be reflected on the host in _/var/db_.

### Using alternative security labeling

You can override the default labeling scheme for each container by specifying
the **--security-opt** flag. For example, you can specify the MCS/MLS level, a
requirement for MLS systems. Specifying the level in the following command
allows you to share the same content between containers.

```
podman run --security-opt label=level:s0:c100,c200 -i -t fedora bash
```

An MLS example might be:

```
$ podman run --security-opt label=level:TopSecret -i -t rhel7 bash
```

To disable the security labeling for this container versus running with the
**--permissive** flag, use the following command:

```
$ podman run --security-opt label=disable -i -t fedora bash
```

If you want a tighter security policy on the processes within a container,
you can specify an alternate type for the container. You could run a container
that is only allowed to listen on Apache ports by executing the following
command:

```
$ podman run --security-opt label=type:svirt_apache_t -i -t centos bash
```

Note you would have to write policy defining a **svirt_apache_t** type.

### Setting device weight

If you want to set _/dev/sda_ device weight to **200**, you can specify the device
weight by **--blkio-weight-device** flag. Use the following command:

```
$ podman run -it --blkio-weight-device "/dev/sda:200" ubuntu
```

### Setting Namespaced Kernel Parameters (Sysctls)

The **--sysctl** sets namespaced kernel parameters (sysctls) in the
container. For example, to turn on IP forwarding in the containers
network namespace, run this command:

```
$ podman run --sysctl net.ipv4.ip_forward=1 someimage
```

Note that not all sysctls are namespaced. Podman does not support changing sysctls
inside of a container that also modify the host system. As the kernel
evolves we expect to see more sysctls become namespaced.

See the definition of the **--sysctl** option above for the current list of
supported sysctls.

### Set UID/GID mapping in a new user namespace

Running a container in a new user namespace requires a mapping of
the uids and gids from the host.

```
$ podman run --uidmap 0:30000:7000 --gidmap 0:30000:7000 fedora echo hello
```

### Configuring Storage Options from the command line

Podman allows for the configuration of storage by changing the values
in the _/etc/container/storage.conf_ or by using global options.  This
shows how to setup and use fuse-overlayfs for a one time run of busybox
using global options.

```
podman --log-level=debug --storage-driver overlay --storage-opt "overlay.mount_program=/usr/bin/fuse-overlayfs" run busybox /bin/sh
```

### Rootless Containers

Podman runs as a non root user on most systems. This feature requires that a new enough version of **shadow-utils**
be installed.  The **shadow-utils** package must include the **newuidmap**(1) and **newgidmap**(1) executables.

Note: RHEL7 and Centos 7 will not have this feature until RHEL7.7 is released.

In order for users to run rootless, there must be an entry for their username in _/etc/subuid_ and _/etc/subgid_ which lists the UIDs for their user namespace.

Rootless Podman works better if the fuse-overlayfs and slirp4netns packages are installed.
The **fuse-overlay** package provides a userspace overlay storage driver, otherwise users need to use
the **vfs** storage driver, which is diskspace expensive and does not perform well. slirp4netns is
required for VPN, without it containers need to be run with the **--network=host** flag.

## ENVIRONMENT

Environment variables within containers can be set using multiple different options,
in the following order of precedence:

- **--env-host**: Host environment of the process executing Podman is added.
- Container image: Any environment variables specified in the container image.
- **--env-file**: Any environment variables specified via env-files.  If multiple files specified, then they override each other in order of entry.
- **--env**: Any environment variables specified will override previous settings.

Run containers and set the environment ending with a __*__ and a __*****__:

```
$ export ENV1=a
$ podman run --env ENV* alpine printenv ENV1
a

$ podman run --env ENV*****=b alpine printenv ENV*****
b
```

## FILES

**/etc/subuid**

**/etc/subgid**

NOTE: Use the environment variable `TMPDIR` to change the temporary storage location of downloaded container images. Podman defaults to use `/var/tmp`.

## SEE ALSO
**subgid**(5), **subuid**(5), **libpod.conf**(5), **systemd.unit**(5), **setsebool**(8), **slirp4netns**(1), **fuse-overlayfs**(1).

## HISTORY
September 2018, updated by Kunal Kushwaha <kushwaha_kunal_v7@lab.ntt.co.jp>

October 2017, converted from Docker documentation to Podman by Dan Walsh for Podman <dwalsh@redhat.com>

November 2015, updated by Sally O'Malley <somalley@redhat.com>

July 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>

June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>

April 2014, Originally compiled by William Henry <whenry@redhat.com> based on docker.com source material and internal work.
