![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Basic Setup and Use of Podman in a Rootless environment.

Prior to allowing users without root privileges to run Podman, the administrator must install or build Podman and complete the following configurations.

## cgroup V2 support

The cgroup V2  Linux kernel feature allows the user to limit the amount of resources a rootless container can use.  If the Linux distribution that you are running Podman on is enabled with  cgroup V2 then you might need to change the default OCI Runtime. Some older versions of `runc` do not work with cgroup V2, you might have to switch to the alternative OCI runtime `crun`.

The alternative OCI runtime support for cgroup V2 can also be turned on at the command line by using the `--runtime` option:

```
podman --runtime crun
```
or for all commands by changing the value for the "Default OCI runtime" in the `containers.conf` file either at the system level or at the [user level](#user-configuration-files) from `runtime = "runc"` to `runtime = "crun"`.

## Administrator Actions

### Installing Podman

For installing Podman, please see the [installation instructions](https://podman.io/getting-started/installation).

### Building Podman

For building Podman, please see the [build instructions](https://podman.io/getting-started/installation#building-from-scratch).

### Install `slirp4netns`

The [slirp4netns](https://github.com/rootless-containers/slirp4netns) package provides user-mode networking for unprivileged network namespaces and must be installed on the machine in order for Podman to run in a rootless environment.  The package is available on most Linux distributions via their package distribution software such as `yum`, `dnf`, `apt`, `zypper`, etc.  If the package is not available, you can build and install `slirp4netns` from [GitHub](https://github.com/rootless-containers/slirp4netns).

### Ensure `fuse-overlayfs` is installed

When using Podman in a rootless environment, it is recommended to use `fuse-overlayfs` rather than the VFS file system. For that you need the `fuse-overlayfs` executable available in `$PATH`.

Your distribution might already provide it in the `fuse-overlayfs` package, but be aware that you need at least version **0.7.6**. This especially needs to be checked on Ubuntu distributions as `fuse-overlayfs` is not generally installed by default and the 0.7.6 version is not available natively on Ubuntu releases prior to **20.04**.

The `fuse-overlayfs` project is available from [GitHub](https://github.com/containers/fuse-overlayfs), and provides instructions for easily building a static `fuse-overlayfs` executable.

If Podman is used before `fuse-overlayfs` is installed, it may be necessary to adjust the `storage.conf` file (see "User Configuration Files" below) to change the `driver` option under `[storage]` to `"overlay"` and point the `mount_program` option in `[storage.options.overlay]` to the path of the `fuse-overlayfs` executable:

```
[storage]
  driver = "overlay"

  (...)

[storage.options.overlay]

  (...)

  mount_program = "/usr/bin/fuse-overlayfs"
```

### Enable user namespaces (on RHEL7 machines)

The number of user namespaces that are allowed on the system is specified in the file `/proc/sys/user/max_user_namespaces`.  On most Linux platforms this is preset by default and no adjustment is necessary.  However, on RHEL7 machines, a user with root privileges may need to set that to a reasonable value by using this command:  `sysctl user.max_user_namespaces=15000`.

### `/etc/subuid` and `/etc/subgid` configuration

Rootless Podman requires the user running it to have a range of UIDs listed in the files `/etc/subuid` and `/etc/subgid`.  The `shadow-utils` or `newuid` package provides these files on different distributions and they must be installed on the system.  Root privileges are required to add or update entries within these files.  The following is a summary from the [How does rootless Podman work?](https://opensource.com/article/19/2/how-does-rootless-podman-work) article by Dan Walsh on [opensource.com](https://opensource.com)

For each user that will be allowed to create containers, update `/etc/subuid` and `/etc/subgid` for the user with fields that look like the following.  Note that the values for each user must be unique.  If there is overlap, there is a potential for a user to use another user's namespace and they could corrupt it.

```
cat /etc/subuid
johndoe:100000:65536
test:165536:65536
```

The format of this file is `USERNAME:UID:RANGE`

* username as listed in `/etc/passwd` or in the output of [`getpwent`](https://man7.org/linux/man-pages/man3/getpwent.3.html).
* The initial UID allocated for the user.
* The size of the range of UIDs allocated for the user.

This means the user `johndoe` is allocated UIDs 100000-165535 as well as their standard UID in the `/etc/passwd` file.  NOTE: this is not currently supported with network installs; these files must be available locally to the host machine.  It is not possible to configure this with LDAP or Active Directory.

If you update either `/etc/subuid` or `/etc/subgid`, you need to stop all the running containers owned by the user and kill the pause process that is running on the system for that user.  This can be done automatically by using the [`podman system migrate`](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-system-migrate.1.md) command which will stop all the containers for the user and will kill the pause process.

Rather than updating the files directly, the `usermod` program can be used to assign UIDs and GIDs to a user.

```
usermod --add-subuids 100000-165535 --add-subgids 100000-165535 johndoe
grep johndoe /etc/subuid /etc/subgid
/etc/subuid:johndoe:100000:65536
/etc/subgid:johndoe:100000:65536
```

### Enable unprivileged `ping`

Users running in a non-privileged container may not be able to use the `ping` utility from that container.

If this is required, the administrator must verify that the UID of the user is part of the range in the `/proc/sys/net/ipv4/ping_group_range` file.

To change its value the administrator can use a call similar to: `sysctl -w "net.ipv4.ping_group_range=0 2000000"`.

To make the change persist, the administrator will need to add a file with the `.conf` file extension in `/etc/sysctl.d` that contains `net.ipv4.ping_group_range=0 $MAX_GID`, where `$MAX_GID` is the highest assignable GID of the user running the container.


## User Actions

The majority of the work necessary to run Podman in a rootless environment is on the shoulders of the machine’s administrator.

Once the Administrator has completed the setup on the machine and then the configurations for the user in `/etc/subuid` and `/etc/subgid`, the user can just start using any Podman command that they wish.

### User Configuration Files

The Podman configuration files for root reside in `/usr/share/containers` with overrides in `/etc/containers`.  In the rootless environment they reside in `${XDG_CONFIG_HOME}/containers` (usually `~/.config/containers`) and are owned by each individual user.

The three main configuration files are [containers.conf](https://github.com/containers/common/blob/main/docs/containers.conf.5.md), [storage.conf](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md) and [registries.conf](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md). The user can modify these files as they wish.

#### containers.conf
Podman reads
1. `/usr/share/containers/containers.conf`
2. `/etc/containers/containers.conf`
3. `$HOME/.config/containers/containers.conf`

if they exist in that order. Each file can override the previous for particular fields.

#### storage.conf
For `storage.conf` the order is
1. `/etc/containers/storage.conf`
2. `$HOME/.config/containers/storage.conf`

In rootless Podman certain fields in `/etc/containers/storage.conf` are ignored. These fields are:
```
graphroot=""
 container storage graph dir (default: "/var/lib/containers/storage")
 Default directory to store all writable content created by container storage programs.

runroot=""
 container storage run dir (default: "/run/containers/storage")
 Default directory to store all temporary writable content created by container storage programs.
```
In rootless Podman these fields default to
```
graphroot="$HOME/.local/share/containers/storage"
runroot="$XDG_RUNTIME_DIR/containers"
```
[$XDG_RUNTIME_DIR](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html#variables) defaults on most systems to `/run/user/$UID`.

#### registries
Registry configuration is read in by this order
1. `/etc/containers/registries.conf`
2. `/etc/containers/registries.d/*`
3. `HOME/.config/containers/registries.conf`

The files in the home directory should be used to configure rootless Podman for personal needs. These files are not created by default. Users can copy the files from `/usr/share/containers` or `/etc/containers` and modify them.

#### Authorization files
 The default authorization file used by the `podman login` and `podman logout` commands reside in `${XDG_RUNTIME_DIR}/containers/auth.json`.

### Using volumes

Rootless Podman is not, and will never be, root; it's not a `setuid` binary, and gains no privileges when it runs. Instead, Podman makes use of a user namespace to shift the UIDs and GIDs of a block of users it is given access to on the host (via the `newuidmap` and `newgidmap` executables) and your own user within the containers that Podman creates.

If your container runs with the root user, then `root` in the container is actually your user on the host. UID/GID 1 is the first UID/GID specified in your user's mapping in `/etc/subuid` and `/etc/subgid`, etc. If you mount a directory from the host into a container as a rootless user, and create a file in that directory as root in the container, you'll see it's actually owned by your user on the host.

So, for example,

```
> whoami
john

# a folder which is empty
host> ls /home/john/folder
host> podman run -v /home/john/folder:/container/volume mycontainer /bin/bash

# Now I'm in the container
root@container> whoami
root
root@container> touch /container/volume/test
root@container> ls -l /container/volume
total 0
-rw-r--r-- 1 root root 0 May 20 21:47 test
root@container> exit

# I check again
host> ls -l /home/john/folder
total 0
-rw-r--r-- 1 john john 0 May 20 21:47 test
```

We do recognize that this doesn't really match how many people intend to use rootless Podman - they want their UID inside and outside the container to match. Thus, we provide the `--userns=keep-id` flag, which ensures that your user is mapped to its own UID and GID inside the container.

It is also helpful to distinguish between running Podman as a rootless user, and a container which is built to run rootless. If the container you're trying to run has a `USER` which is not root, then when mounting volumes you **must** use `--userns=keep-id`. This is because the container user would not be able to become `root` and access the mounted volumes.

Another consideration in regards to volumes:

- When providing the path of a directory you'd like to bind-mount, the path needs to be provided as an absolute path
  or a relative path that starts with `.` (a dot), otherwise the string will be interpreted as the name of a named volume.

## More information

If you are still experiencing problems running Podman in a rootless environment, please refer to the [Shortcomings of Rootless Podman](https://github.com/containers/podman/blob/main/rootless.md) page which lists known issues and solutions to known issues in this environment.

For more information on Podman and its subcommands, follow the links on the main [README.md](../../README.md#podman-information-for-developers) page or the [podman.io](https://podman.io) web site.
