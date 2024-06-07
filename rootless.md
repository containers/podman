# Shortcomings of Rootless Podman

The following list categorizes the known issues and irregularities with running Podman as a non-root user.  Although currently functional, there is still a number of work items that are under consideration to be added.  These proposed changes are in varying degrees of design and development.

Contributors are more than welcomed to help with this work.  If you decide to carve off a piece and work on it, please create an issue in [GitHub](https://github.com/containers/podman/issues), and assign it to yourself.  If you find other unexpected behavior with rootless Podman and feel it’s warranted, please feel free to update this document.

* Podman can not create containers that bind to ports < 1024.
  * The kernel does not allow processes without CAP_NET_BIND_SERVICE to bind to low ports.
  * You can modify the `net.ipv4.ip_unprivileged_port_start` sysctl to change the lowest port.  For example `sysctl net.ipv4.ip_unprivileged_port_start=443` allows rootless Podman containers to bind to ports >= 443.
  * A proxy server, kernel firewall rule, or redirection tool such as [redir](https://github.com/troglobit/redir) may be used to redirect traffic from a privileged port to an unprivileged one (where a podman pod is bound) in a server scenario - where a user has access to the root account (or setuid on the binary would be an acceptable risk), but wants to run the containers as an unprivileged user for enhanced security and for a limited number of pre-known ports.
* As of Podman 5.0, pasta is the default networking tool. Since pasta copies the IP address of the main interface, connections to that IP from containers do not work. This means that unless you have more than one interface, inter-container connections cannot be made without explicitly passing a pasta network configuration, either in `containers.conf` or at runtime.
  * If you previously had port forwards (ex. via -p 80:80) that other containers could access, you can either revert back to slirp4netns or use the solution (setting pasta options with 10.0.2.x IPs) posted [here](https://blog.podman.io/2024/03/podman-5-0-breaking-changes-in-detail/).
* “How To” documentation is patchy at best.
* If /etc/subuid and /etc/subgid are not set up for a user, then podman commands
can easily fail
  * This can be a big issue on machines using Network Based Password information (FreeIPA, Active Directory, LDAP)
  * We are working to get support for NSSWITCH on the /etc/subuid and /etc/subgid files.
* No cgroup V1 Support
  * cgroup V1 does not safely support cgroup delegation.
  * As of Fedora 31 defaults to cgroup V2, which has full support of rootless cgroup management.  Note this requires the --cgroup-manager within rootless containers to use systemd, which new containers will get by default.
* Some system unit configuration options do not work in the rootless container
  * systemd fails to apply several options and failures are silently ignored (e.g. CPUShares, MemoryLimit). Should work on cgroup V2.
  * Use of certain options will cause service startup failures (e.g. PrivateNetwork).  The systemd services requiring `PrivateNetwork` can be made to work by passing `--cap-add SYS_ADMIN`, but the security implications should be carefully evaluated.  In most cases, it's better to create an override.conf drop-in that sets `PrivateNetwork=no`.  This also applies to containers run by root.
* Can not share container images with CRI-O or other rootful users
* Difficult to use additional stores for sharing content
* Does not work on NFS or parallel filesystem homedirs (e.g. [GPFS](https://www.ibm.com/support/knowledgecenter/en/SSFKCN/gpfs_welcome.html))
  * NFS and parallel filesystems enforce file creation on different UIDs on the server side and does not understand User Namespace.
  * When a container root process like YUM attempts to create a file owned by a different UID, NFS Server/GPFS denies the creation.
* Does not work with homedirs mounted with noexec/nodev
  * User can set up storage to point to other directories they can write to that are not mounted noexec/nodev
* Support for using native overlayfs as an unprivileged user is only available for Podman version >= 3.1 on a Linux kernel version >= 5.12, otherwise the slower _fuse-overlayfs_ may be used.
  * A few Linux distributions (e.g. Ubuntu) have supported even older Podman and Linux kernel versions by modifying the normal Linux kernel behaviour.
* Only other supported driver is VFS.
* Cannot use ping out of the box.
  * [(Can be fixed by setting sysctl on host)](https://github.com/containers/podman/blob/main/troubleshooting.md#5-rootless-containers-cannot-ping-hosts)
* Requires new shadow-utils (not found in older (RHEL7/Centos7 distros) Should be fixed in RHEL7.7 release)
* A few commands do not work.
  * mount/unmount (on fuse-overlay)
     * Only works if you enter the mount namespace with podman unshare
  * podman stats (Works with cgroup V2 support)
  * Checkpoint and Restore (CRIU requires root)
  * Pause and Unpause (Works with cgroup V2 support)
* Issues with higher UIDs can cause builds to fail
  * If a build is attempting to use a UID that is not mapped into the user namespace mapping for a container, then builds will not be able to put the UID in an image.
* Making device nodes within a container fails, even when running --privileged.
  * The kernel does not allow non root user processes (processes without CAP_MKNOD) to create device nodes.  If a container needs to create device nodes, it must be run as root.
* When using --net=host with rootless containers, subsequent podman execs to that container will not join the host network namespace because it is owned by root.
