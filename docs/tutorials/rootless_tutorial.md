![PODMAN logo](../../logo/podman-logo-source.svg)

# Basic Setup and Use of Podman in a Rootless environment.

Prior to allowing users without root privileges to run Podman, the administrator must install or build Podman and complete the following configurations.

## Administrator Actions

### Installing Podman

For installing Podman, please see the [installation instructions](https://github.com/containers/libpod/blob/master/install.md).

### Building Podman

For building Podman, please see the [installation instructions](https://github.com/containers/libpod/blob/master/install.md#building-from-scratch).

### Install slirp4netns

The [slirp4netns](https://github.com/rootless-containers/slirp4netns) package provides user-mode networking for unprivileged network namespaces and must be installed on the machine in order for Podman to run in a rootless environment.  The package is available on most Linux distributions via their package distribution software such as yum, dnf, apt, zypper, etc.  If the package is not available, you can build and install slirp4netns from [GitHub](https://github.com/rootless-containers/slirp4netns).

### Ensure fuse-overlayfs is installed

When using Podman in a rootless environment, it is recommended to use fuse-overlayfs rather than the VFS file system.  Installing the fuse3-devel package gives Podman the dependencies it needs to install, build and use fuse-overlayfs in a rootless environment for you.  The fuse-overlayfs project is also available from [GitHub](https://github.com/containers/fuse-overlayfs).  This especially needs to be checked on Ubuntu distributions as fuse-overlayfs is not generally installed by default.

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

If you update either the /etc/subuid or the /etc/subgid file, you need to stop all the running containers owned by the user and kill the pause process that is running on the system for that user.  This can be done automatically by using the `[podman system migrate](https://github.com/containers/libpod/blob/master/docs/podman-system-migrate.1.md)` command which will stop all the containers for the user and will kill the pause process.

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

### User Configuration Files.

The Podman configuration files for root reside in /etc/containers.  In the rootless environment they reside in ${XDG_RUNTIME_DIR}/containers and are owned by each individual user.  The user can modify these files as they wish.

## More information

If you are still experiencing problems running Podman in a rootless environment, please refer to the [Shortcomings of Rootless Podman](https://github.com/containers/libpod/blob/master/rootless.md) page which lists known issues and solutions to known issues in this environment.

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands) page or the [podman.io](https://podman.io) web site.
