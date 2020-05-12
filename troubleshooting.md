![PODMAN logo](logo/podman-logo-source.svg)

# Troubleshooting

## A list of common issues and solutions for Podman

---
### 1) Variety of issues - Validate Version

A large number of issues reported against Podman are often found to already be fixed
in more current versions of the project.  Before reporting an issue, please verify the
version you are running with `podman version` and compare it to the latest release
documented on the top of Podman's [README.md](README.md).

If they differ, please update your version of PODMAN to the latest possible
and retry your command before reporting the issue.

---
### 2) Can't use volume mount, get permission denied

$ podman run -v ~/mycontent:/content fedora touch /content/file
touch: cannot touch '/content/file': Permission denied

#### Solution

This is usually caused by SELinux.

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

$ podman run -v ~/mycontent:/content:Z fedora touch /content/file

Make sure the content is private for the container.  Do not relabel system directories and content.
Relabeling system content might cause other confined services on your machine to fail.  For these
types of containers we recommmend that disable SELinux separation.  The option `--security-opt label=disable`
will disable SELinux separation for the container.

$ podman run --security-opt label=disable -v ~:/home/user fedora touch /home/user/file

---
### 3) No such image or Bare keys cannot contain ':'

When doing a `podman pull` or `podman build` command and a "common" image cannot be pulled,
it is likely that the `/etc/containers/registries.conf` file is either not installed or possibly
misconfigured.

#### Symptom

```console
$ sudo podman build -f Dockerfile
STEP 1: FROM alpine
error building: error creating build container: no such image "alpine" in registry: image not known
```

or

```console
$ sudo podman pull fedora
error pulling image "fedora": unable to pull fedora: error getting default registries to try: Near line 9 (last key parsed ''): Bare keys cannot contain ':'.
```

#### Solution

  * Verify that the `/etc/containers/registries.conf` file exists.  If not, verify that the containers-common package is installed.
  * Verify that the entries in the `[registries.search]` section of the /etc/containers/registries.conf file are valid and reachable.
    *  i.e. `registries = ['registry.fedoraproject.org', 'quay.io', 'registry.access.redhat.com']`

---
### 4) http: server gave HTTP response to HTTPS client

When doing a Podman command such as `build`, `commit`, `pull`, or `push` to a registry,
tls verification is turned on by default.  If authentication is not used with
those commands, this error can occur.

#### Symptom

```console
$ sudo podman push alpine docker://localhost:5000/myalpine:latest
Getting image source signatures
Get https://localhost:5000/v2/: http: server gave HTTP response to HTTPS client
```

#### Solution

By default tls verification is turned on when communicating to registries from
Podman.  If the registry does not require authentication the Podman commands
such as `build`, `commit`, `pull` and `push` will fail unless tls verification is turned
off using the `--tls-verify` option.  **NOTE:** It is not at all recommended to
communicate with a registry and not use tls verification.

  * Turn off tls verification by passing false to the tls-verification option.
  * I.e. `podman push --tls-verify=false alpine docker://localhost:5000/myalpine:latest`

---
### 5) Rootless: could not get runtime - database configuration mismatch

In Podman release 0.11.1, a default path for rootless containers was changed,
potentially causing rootless Podman to be unable to function. The new default
path is not a problem for new installations, but existing installations will
need to work around it with the following fix.

#### Symptom

```console
$ podman info
could not get runtime: database run root /run/user/1000/run does not match our run root /run/user/1000: database configuration mismatch
```

#### Solution

This problem has been fixed in Podman release 0.12.1 and it is recommended
to upgrade to that version.  If that is not possible use the following procedure.

To work around the new default path, we can manually set the path Podman is
expecting in a configuration file.

First, we need to make a new local configuration file for rootless Podman.
* `mkdir -p ~/.config/containers`
* `cp /usr/share/containers/libpod.conf ~/.config/containers`

Next, edit the new local configuration file
(`~/.config/containers/libpod.conf`) with your favorite editor. Comment out the
line starting with `cgroup_manager` by adding a `#` character at the beginning
of the line, and change the path in the line starting with `tmp_dir` to point to
the first path in the error message Podman gave (in this case,
`/run/user/1000/tmp`).

---
### 6) rootless containers cannot ping hosts

When using the ping command from a non-root container, the command may
fail because of a lack of privileges.

#### Symptom

```console
$ podman run --rm fedora ping -W10 -c1 redhat.com
PING redhat.com (209.132.183.105): 56 data bytes

--- redhat.com ping statistics ---
1 packets transmitted, 0 packets received, 100% packet loss
```

#### Solution

It is most likely necessary to enable unprivileged pings on the host.
Be sure the UID of the user is part of the range in the
`/proc/sys/net/ipv4/ping_group_range` file.

To change its value you can use something like: `sysctl -w
"net.ipv4.ping_group_range=0 2000000"`.

To make the change persistent, you'll need to add a file in
`/etc/sysctl.d` that contains `net.ipv4.ping_group_range=0 $MAX_UID`.

---
### 7) Build hangs when the Dockerfile contains the useradd command

When the Dockerfile contains a command like `RUN useradd -u 99999000 -g users newuser` the build can hang.

#### Symptom

If you are using a useradd command within a Dockerfile with a large UID/GID, it will create a large sparse file `/var/log/lastlog`.  This can cause the build to hang forever.  Go language does not support sparse files correctly, which can lead to some huge files being created in your container image.

#### Solution

If the entry in the Dockerfile looked like: RUN useradd -u 99999000 -g users newuser then add the `--no-log-init` parameter to change it to: `RUN useradd --no-log-init -u 99999000 -g users newuser`. This option tells useradd to stop creating the lastlog file.

### 8) Permission denied when running Podman commands

When rootless Podman attempts to execute a container on a non exec home directory a permission error will be raised.

#### Symptom

If you are running Podman or buildah on a home directory that is mounted noexec,
then they will fail. With a message like:

```
podman run centos:7
standard_init_linux.go:203: exec user process caused "permission denied"
```

#### Solution

Since the administrator of the system setup your home directory to be noexec, you will not be allowed to execute containers from storage in your home directory. It is possible to work around this by manually specifying a container storage path that is not on a noexec mount. Simply copy the file /etc/containers/storage.conf to ~/.config/containers/ (creating the directory if necessary). Specify a graphroot directory which is not on a noexec mount point and to which you have read/write privileges.  You will need to modify other fields to writable directories as well.

For example

```
cat ~/.config/containers/storage.conf
[storage]
  driver = "overlay"
  runroot = "/run/user/1000"
  graphroot = "/execdir/myuser/storage"
  [storage.options]
    mount_program = "/bin/fuse-overlayfs"
```

### 9) Permission denied when running systemd within a Podman container

When running systemd as PID 1 inside of a container on an SELinux
separated machine, it needs to write to the cgroup file system.

#### Symptom

Systemd gets permission denied when attempting to write to the cgroup file
system, and AVC messages start to show up in the audit.log file or journal on
the system.

#### Solution

Newer versions of Podman (2.0 or greater) support running init based containers
with a different SELinux labels, which allow the container process access to the
cgroup file system. This feature requires container-selinux-2.132 or newer
versions.

Prior to Podman 2.0, the SELinux boolean `container_manage_cgroup` allows
container processes to write to the cgroup file system. Turn on this boolean,
on SELinux separated systems, to allow systemd to run properly in the container.
Only do this on systems running older versions of Podman.

`setsebool -P container_manage_cgroup true`

### 10) Newuidmap missing when running rootless Podman commands

Rootless Podman requires the newuidmap and newgidmap programs to be installed.

#### Symptom

If you are running Podman or buildah as a not root user, you get an error complaining about
a missing newuidmap executable.

```
podman run -ti fedora sh
cannot find newuidmap: exec: "newuidmap": executable file not found in $PATH
```

#### Solution

Install a version of shadow-utils that includes these executables.  Note RHEL 7 and CentOS 7 will not have support for this until RHEL7.7 is released.

### 11) rootless setup user: invalid argument

Rootless Podman requires the user running it to have a range of UIDs listed in /etc/subuid and /etc/subgid.

#### Symptom

An user, either via --user or through the default configured for the image, is not mapped inside the namespace.

```
podman run --rm -ti --user 1000000 alpine echo hi
Error: container create failed: container_linux.go:344: starting container process caused "setup user: invalid argument"
```

#### Solution

Update the /etc/subuid and /etc/subgid with fields for users that look like:

```
cat /etc/subuid
johndoe:100000:65536
test:165536:65536
```

The format of this file is USERNAME:UID:RANGE

* username as listed in /etc/passwd or getpwent.
* The initial uid allocated for the user.
* The size of the range of UIDs allocated for the user.

This means johndoe is allocated UIDS 100000-165535 as well as his standard UID in the
/etc/passwd file.

You should ensure that each user has a unique range of uids, because overlapping UIDs,
would potentially allow one user to attack another user.

You could also use the usermod program to assign UIDs to a user.

If you update either the /etc/subuid or /etc/subgid file, you need to
stop all running containers and kill the pause process.  This is done
automatically by the `system migrate` command, which can also be used
to stop all the containers and kill the pause process.

```
usermod --add-subuids 200000-201000 --add-subgids 200000-201000 johndoe
grep johndoe /etc/subuid /etc/subgid
/etc/subuid:johndoe:200000:1001
/etc/subgid:johndoe:200000:1001
```

### 12) Changing the location of the Graphroot leads to permission denied

When I change the graphroot storage location in storage.conf, the next time I
run Podman I get an error like:

```
# podman run -p 5000:5000 -it centos bash

bash: error while loading shared libraries: /lib64/libc.so.6: cannot apply additional memory protection after relocation: Permission denied
```

For example, the admin sets up a spare disk to be mounted at `/src/containers`,
and points storage.conf at this directory.


#### Symptom

SELinux blocks containers from using random locations for overlay storage.
These directories need to be labeled with the same labels as if the content was
under /var/lib/containers/storage.

#### Solution

Tell SELinux about the new containers storage by setting up an equivalence record.
This tells SELinux to label content under the new path, as if it was stored
under `/var/lib/containers/storage`.

```
semanage fcontext -a -e /var/lib/containers /srv/containers
restorecon -R -v /srv/containers
```

The semanage command above tells SELinux to setup the default labeling of
`/srv/containers` to match `/var/lib/containers`.  The `restorecon` command
tells SELinux to apply the labels to the actual content.

Now all new content created in these directories will automatically be created
with the correct label.

### 13) Anonymous image pull fails with 'invalid username/password'

Pulling an anonymous image that doesn't require authentication can result in an
`invalid username/password` error.

#### Symptom

If you pull an anonymous image, one that should not require credentials, you can receive
and `invalid username/password` error if you have credentials established in the
authentication file for the target container registry that are no longer valid.

```
podman run -it --rm docker://docker.io/library/alpine:latest ls
Trying to pull docker://docker.io/library/alpine:latest...ERRO[0000] Error pulling image ref //alpine:latest: Error determining manifest MIME type for docker://alpine:latest: unable to retrieve auth token: invalid username/password
Failed
Error: unable to pull docker://docker.io/library/alpine:latest: unable to pull image: Error determining manifest MIME type for docker://alpine:latest: unable to retrieve auth token: invalid username/password
```

This can happen if the authentication file is modified 'by hand' or if the credentials
are established locally and then the password is updated later in the container registry.

#### Solution

Depending upon which container tool was used to establish the credentials, use `podman logout`
or `docker logout` to remove the credentials from the authentication file.

### 14) Running Podman inside a container causes container crashes and inconsistent states

Running Podman in a container and forwarding some, but not all, of the required host directories can cause inconsistent container behavior.

#### Symptom

After creating a container with Podman's storage directories mounted in from the host and running Podman inside a container, all containers show their state as "configured" or "created", even if they were running or stopped.

#### Solution

When running Podman inside a container, it is recommended to mount at a minimum `/var/lib/containers/storage/` as a volume.
Typically, you will not mount in the host version of the directory, but if you wish to share containers with the host, you can do so.
If you do mount in the host's `/var/lib/containers/storage`, however, you must also mount in the host's `/var/run/libpod` and `/var/run/containers/storage` directories.
Not doing this will cause Podman in the container to detect that temporary files have been cleared, leading it to assume a system restart has taken place.
This can cause Podman to reset container states and lose track of running containers.

For running containers on the host from inside a container, we also recommend the [Podman remote client](remote_client.md), which only requires a single socket to be mounted into the container.

### 15) Rootless 'podman build' fails EPERM on NFS:

NFS enforces file creation on different UIDs on the server side and does not understand user namespace, which rootless Podman requires.
When a container root process like YUM attempts to create a file owned by a different UID, NFS Server denies the creation.
NFS is also a problem for the file locks when the storage is on it.  Other distributed file systems (for example: Lustre, Spectrum Scale, the General Parallel File System (GPFS)) are also not supported when running in rootless mode as these file systems do not understand user namespace.

#### Symptom
```console
$ podman build .
ERRO[0014] Error while applying layer: ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
error creating build container: Error committing the finished image: error adding layer with blob "sha256:a02a4930cb5d36f3290eb84f4bfa30668ef2e9fe3a1fb73ec015fc58b9958b17": ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
```

#### Solution
Choose one of the following:
  * Setup containers/storage in a different directory, not on an NFS share.
    * Create a directory on a local file system.
    * Edit `~/.config/containers/libpod.conf` and point the `volume_path` option to that local directory.
  * Otherwise just run Podman as root, via `sudo podman`

### 16) Rootless 'podman build' fails when using OverlayFS:

The Overlay file system (OverlayFS) requires the ability to call the `mknod` command when creating whiteout files
when extracting an image.  However, a rootless user does not have the privileges to use `mknod` in this capacity.

#### Symptom
```console
podman build --storage-driver overlay .
STEP 1: FROM docker.io/ubuntu:xenial
Getting image source signatures
Copying blob edf72af6d627 done
Copying blob 3e4f86211d23 done
Copying blob 8d3eac894db4 done
Copying blob f7277927d38a done
Copying config 5e13f8dd4c done
Writing manifest to image destination
Storing signatures
Error: error creating build container: Error committing the finished image: error adding layer with blob "sha256:8d3eac894db4dc4154377ad28643dfe6625ff0e54bcfa63e0d04921f1a8ef7f8": Error processing tar file(exit status 1): operation not permitted
$ podman build .
ERRO[0014] Error while applying layer: ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
error creating build container: Error committing the finished image: error adding layer with blob "sha256:a02a4930cb5d36f3290eb84f4bfa30668ef2e9fe3a1fb73ec015fc58b9958b17": ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
```

#### Solution
Choose one of the following:
  * Complete the build operation as a privileged user.
  * Install and configure fuse-overlayfs.
    * Install the fuse-overlayfs package for your Linux Distribution.
    * Add `mount_program = "/usr/bin/fuse-overlayfs"` under `[storage.options]` in your `~/.config/containers/storage.conf` file.

### 17) RHEL 7 and CentOS 7 based `init` images don't work with cgroup v2

The systemd version shipped in RHEL 7 and CentOS 7 doesn't have support for cgroup v2.  Support for cgroup V2 requires version 230 of systemd or newer, which
was never shipped or supported on RHEL 7 or CentOS 7.

#### Symptom
```console

sh# podman run --name test -d registry.access.redhat.com/rhel7-init:latest && sleep 10 && podman exec test systemctl status
c8567461948439bce72fad3076a91ececfb7b14d469bfa5fbc32c6403185beff
Failed to get D-Bus connection: Operation not permitted
Error: non zero exit code: 1: OCI runtime error
```

#### Solution
You'll need to either:

* configure the host to use cgroup v1

```
On Fedora you can do:
# dnf install -y grubby
# grubby --update-kernel=ALL --args=”systemd.unified_cgroup_hierarchy=0"
# reboot
```

* update the image to use an updated version of systemd.

### 18) rootless containers exit once the user session exits

You need to set lingering mode through loginctl to prevent user processes to be killed once
the user session completed.

#### Symptom

Once the user logs out all the containers exit.

#### Solution
You'll need to either:

* loginctl enable-linger $UID

or as root if your user has not enough privileges.

* sudo loginctl enable-linger $UID

### 19) `podman run` fails with "bpf create: permission denied error"

The Kernel Lockdown patches deny eBPF programs when Secure Boot is enabled in the BIOS. [Matthew Garrett's post](https://mjg59.dreamwidth.org/50577.html) describes the relationship between Lockdown and Secure Boot and [Jan-Philip Gehrcke's](https://gehrcke.de/2019/09/running-an-ebpf-program-may-require-lifting-the-kernel-lockdown/) connects this with eBPF. [RH bug 1768125](https://bugzilla.redhat.com/show_bug.cgi?id=1768125) contains some additional details.

#### Symptom

Attempts to run podman result in

```Error: bpf create : Operation not permitted: OCI runtime permission denied error```

#### Solution

One workaround is to disable Secure Boot in your BIOS.

### 20) error creating libpod runtime: there might not be enough IDs available in the namespace

Unable to pull images

#### Symptom

```console
$ podman unshare cat /proc/self/uid_map
	 0       1000          1
```

#### Solution

```console
$ podman system migrate
```

Original command now returns

```
$ podman unshare cat /proc/self/uid_map
	 0       1000          1
	 1     100000      65536
```

Reference [subuid](http://man7.org/linux/man-pages/man5/subuid.5.html) and [subgid](http://man7.org/linux/man-pages/man5/subgid.5.html) man pages for more detail.

### 21) Passed-in device can't be accessed in rootless container

As a non-root user you have group access rights to a device that you want to
pass into a rootless container with `--device=...`.

#### Symptom

Any access inside the container is rejected with "Permission denied".

#### Solution

The runtime uses `setgroups(2)` hence the process looses all additional groups
the non-root user has. If you use the `crun` runtime, 0.10.4 or newer,
then you can enable a workaround by adding `--annotation io.crun.keep_original_groups=1`
to the `podman` command line.

### 22) A rootless container running in detached mode is closed at logout

When running a container with a command like `podman run --detach httpd` as
a rootless user, the container is closed upon logout and is not kept running.

#### Symptom

When logging out of a rootless user session, all containers that were started
in detached mode are stopped and are not kept running.  As the root user, these
same containers would survive the logout and continue running.

#### Solution

When systemd notes that a session that started a Podman container has exited,
it will also stop any containers that has been associated with it.  To avoid
this, use the following command before logging out: `loginctl enable-linger`.
To later revert the linger functionality, use `loginctl disable-linger`.

LOGINCTL(1), SYSTEMD(1)
