![PODMAN logo](../../logo/podman-logo-source.svg)

# Basic Setup and Use of Podman in a Rootless environment.

Prior to allowing users without root privileges to run Podman, the administrator must install or build Podman and complete the following configurations.

## cgroup V2 support

The cgroup V2  Linux kernel feature allows the user to limit the amount of resources a rootless container can use.  If the Linux distribution that you are running Podman on is enabled with  cgroup V2 then you might need to change the default OCI Runtime.  The default runtime `runc` does not currently work with cgroup V2 enabled systems, so you have to switch to the alternative OCI runtime `crun`.

The alternative OCI runtime support for cgroup V2 can  be turned on at the command line by using the `--runtime` option:

```
sudo podman --runtime /usr/bin/crun
```
or by changing the value for the "Default OCI runtime" in the libpod.conf file either at the system level or at the [user level](#user-configuration-files) from `runtime = "runc"` to `runtime = "crun"`.

## Administrator Actions

### Installing Podman

For installing Podman, please see the [installation instructions](https://github.com/containers/libpod/blob/master/install.md).

### Building Podman

For building Podman, please see the [installation instructions](https://github.com/containers/libpod/blob/master/install.md#building-from-scratch).

### Install slirp4netns

The [slirp4netns](https://github.com/rootless-containers/slirp4netns) package provides user-mode networking for unprivileged network namespaces and must be installed on the machine in order for Podman to run in a rootless environment.  The package is available on most Linux distributions via their package distribution software such as yum, dnf, apt, zypper, etc.  If the package is not available, you can build and install slirp4netns from [GitHub](https://github.com/rootless-containers/slirp4netns).

### Ensure fuse-overlayfs is installed

When using Podman in a rootless environment, it is recommended to use fuse-overlayfs rather than the VFS file system. For that you need the `fuse-overlayfs` executable available in `$PATH`.

Your distribution might already provide it in the `fuse-overlayfs` package, but be aware that you need at least version **0.7.6**. This especially needs to be checked on Ubuntu distributions as `fuse-overlayfs` is not generally installed by default and the 0.7.6 version is not available natively on Ubuntu releases prior to **20.04**.

The fuse-overlayfs project is available from [GitHub](https://github.com/containers/fuse-overlayfs), and provides instructions for easily building a static `fuse-overlayfs` executable.

If Podman is used before fuse-overlayfs is installed, it may be necessary to adjust the `storage.conf` file (see "User Configuration Files" below) to change the `driver` option under `[storage]` to `"overlay"` and point the `mount_program` option in `[storage.options]` to the path of the `fuse-overlayfs` executable:

```
[storage]
  driver = "overlay"

  (...)

  [storage.options]

    (...)

    mount_program = "/usr/bin/fuse-overlayfs"
```

### Enable user namespaces (on RHEL7 machines)

The number of user namespaces that are allowed on the system is specified in the file `/proc/sys/user/max_user_namespaces`.  On most Linux platforms this is preset by default and no adjustment is necessary.  However on RHEL7 machines a user with root privileges may need to set that to a reasonable value by using this command:  `sysctl user.max_user_namespaces=15000`.

### /etc/subuid and /etc/subgid configuration

Rootless podman requires the user running it to have a range of UIDs listed in /etc/subuid and /etc/subgid files.  The `shadows-utils` or `newuid` package provides these files on different distributions and  they must be installed on the system.  These files will need someone with root privileges on the system to add or update the entries within them.  The following is a summarization from the [How does rootless Podman work?](https://opensource.com/article/19/2/how-does-rootless-podman-work) article by Dan Walsh on [opensource.com](https://opensource.com)

Update the /etc/subuid and /etc/subgid with fields for each user that will be allowed to create containers that look like the following.  Note that the values for each user must be unique and without any overlap.  If there is an overlap, there is a potential for a user to use another’s namespace and they could corrupt it.

```
cat /etc/subuid
johndoe:100000:65536
test:165536:65536
```

The format of this file is USERNAME:UID:RANGE

* username as listed in /etc/passwd or getpwent.
* The initial uid allocated for the user.
* The size of the range of UIDs allocated for the user.

This means the user johndoe is allocated UIDS 100000-165535 as well as their standard UID in the /etc/passwd file.  NOTE: this is not currently supported with network installs.  These files must be available locally to the host machine.  It is not possible to configure this with LDAP or Active Directory.

If you update either the /etc/subuid or the /etc/subgid file, you need to stop all the running containers owned by the user and kill the pause process that is running on the system for that user.  This can be done automatically by using the [`podman system migrate`](https://github.com/containers/libpod/blob/master/docs/podman-system-migrate.1.md) command which will stop all the containers for the user and will kill the pause process.

Rather than updating the files directly, the usermod program can be used to assign UIDs and GIDs to a user.

```
usermod --add-subuids 200000-201000 --add-subgids 200000-201000 johndoe
grep johndoe /etc/subuid /etc/subgid
/etc/subuid:johndoe:200000:1001
/etc/subgid:johndoe:200000:1001
```

### Enable unprivileged `ping`

Users running in a non-privileged container may not be able to use the `ping` utility from that container.

If this is required, the administrator must verify that the UID of the user is part of the range in the `/proc/sys/net/ipv4/ping_group_range` file.

To change its value the administrator can use a call similar to: `sysctl -w "net.ipv4.ping_group_range=0 2000000"`.

To make the change persistent, the administrator will need to add a file in `/etc/sysctl.d` that contains `net.ipv4.ping_group_range=0 $MAX_UID`.


## User Actions

The majority of the work necessary to run Podman in a rootless environment is on the shoulders of the machine’s administrator.

Once the Administrator has completed the setup on the machine and then the configurations for the user in /etc/subuid and /etc/subgid, the user can just start using any Podman command that they wish.

### User Configuration Files

The Podman configuration files for root reside in `/usr/share/containers` with overrides in `/etc/containers`.  In the rootless environment they reside in `${XDG_CONFIG_HOME}/containers` (usually `~/.config/containers`) and are owned by each individual user.  The main files are `libpod.conf` and `storage.conf` and the user can modify these files as they wish.

The default authorization file used by the `podman login` and `podman logout` commands reside in `${XDG_RUNTIME_DIR}/containers/auth.json`.

### Using volumes

Rootless Podman is not, and will never be, root; it's not a setuid binary, and gains no privileges when it runs. Instead, Podman makes use of a user namespace to shift the UIDs and GIDs of a block of users it is given access to on the host (via the newuidmap and newgidmap executables) and your own user within the containers that podman creates.

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

It is also helpful to distinguish between running podman as a rootless user, and a container which is built to run rootless. If the container you're trying you run has a `USER` which is not root, then when mounting volumes you **must** use `--userns=keep-id`. This is because the container user would not be able to become `root` and access the mounted volumes.

Other considerations in regards to volumes:

- You should always give the full path to the volume you'd like to mount
- The mount point must exist in the container

## More information

If you are still experiencing problems running Podman in a rootless environment, please refer to the [Shortcomings of Rootless Podman](https://github.com/containers/libpod/blob/master/rootless.md) page which lists known issues and solutions to known issues in this environment.

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands) page or the [podman.io](https://podman.io) web site.
