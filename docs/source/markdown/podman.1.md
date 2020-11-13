% podman(1)

## NAME
podman - Simple management tool for pods, containers and images

## SYNOPSIS
**podman** [*options*] *command*

## DESCRIPTION
Podman (Pod Manager) is a fully featured container engine that is a simple daemonless tool.
Podman provides a Docker-CLI comparable command line that eases the transition from other
container engines and allows the management of pods, containers and images.  Simply put: `alias docker=podman`.
Most Podman commands can be run as a regular user, without requiring additional
privileges.

Podman uses Buildah(1) internally to create container images. Both tools share image
(not container) storage, hence each can use or manipulate images (but not containers)
created by the other.

**podman [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

#### **--cgroup-manager**=*manager*

The CGroup manager to use for container cgroups. Supported values are cgroupfs or systemd. Default is systemd unless overridden in the containers.conf file.

Note: Setting this flag can cause certain commands to break when called on containers previously created by the other CGroup manager type.
Note: CGroup manager is not supported in rootless mode when using CGroups Version V1.

#### **--cni-config-dir**
Path of the configuration directory for CNI networks.  (Default: `/etc/cni/net.d`)

#### **--connection**, **-c**
Connection to use for remote podman (Default connection is configured in `containers.conf`)

#### **--conmon**
Path of the conmon binary (Default path is configured in `containers.conf`)

#### **--events-backend**=*type*

Backend to use for storing events. Allowed values are **file**, **journald**, and **none**.

#### **--help**, **-h**

Print usage statement

#### **--hooks-dir**=*path*

Each `*.json` file in the path configures a hook for Podman containers.  For more details on the syntax of the JSON files and the semantics of hook injection, see `oci-hooks(5)`.  Podman and libpod currently support both the 1.0.0 and 0.1.0 hook schemas, although the 0.1.0 schema is deprecated.

This option may be set multiple times; paths from later options have higher precedence (`oci-hooks(5)` discusses directory precedence).

For the annotation conditions, libpod uses any annotations set in the generated OCI configuration.

For the bind-mount conditions, only mounts explicitly requested by the caller via `--volume` are considered.  Bind mounts that libpod inserts by default (e.g. `/dev/shm`) are not considered.

If `--hooks-dir` is unset for root callers, Podman and libpod will currently default to `/usr/share/containers/oci/hooks.d` and `/etc/containers/oci/hooks.d` in order of increasing precedence.  Using these defaults is deprecated, and callers should migrate to explicitly setting `--hooks-dir`.

Podman and libpod currently support an additional `precreate` state which is called before the runtime's `create` operation.  Unlike the other stages, which receive the container state on their standard input, `precreate` hooks receive the proposed runtime configuration on their standard input.  They may alter that configuration as they see fit, and write the altered form to their standard output.

**WARNING**: the `precreate` hook lets you do powerful things, such as adding additional mounts to the runtime configuration.  That power also makes it easy to break things.  Before reporting libpod errors, try running your container with `precreate` hooks disabled to see if the problem is due to one of your hooks.

#### **--identity**=*path*

Path to ssh identity file. If the identity file has been encrypted, podman prompts the user for the passphrase.
If no identity file is provided and no user is given, podman defaults to the user running the podman command.
Podman prompts for the login password on the remote server.

Identity value resolution precedence:
 - command line value
 - environment variable `CONTAINER_SSHKEY`, if `CONTAINER_HOST` is found
 - `containers.conf`

#### **--log-level**=*level*

Log messages above specified level: debug, info, warn, error (default), fatal or panic (default: "error")

#### **--namespace**=*namespace*

Set libpod namespace. Namespaces are used to separate groups of containers and pods in libpod's state.
When namespace is set, created containers and pods will join the given namespace, and only containers and pods in the given namespace will be visible to Podman.

#### **--network-cmd-path**=*path*
Path to the command binary to use for setting up a network.  It is currently only used for setting up a slirp4netns network.  If "" is used then the binary is looked up using the $PATH environment variable.

#### **--remote**, **-r**
Access Podman service will be remote

#### **--url**=*value*
URL to access Podman service (default from `containers.conf`, rootless `unix://run/user/$UID/podman/podman.sock` or as root `unix://run/podman/podman.sock`).

 - `CONTAINER_HOST` is of the format `<schema>://[<user[:<password>]@]<host>[:<port>][<path>]`

Details:
 - `user` will default to either `root` or current running user
 - `password` has no default
 - `host` must be provided and is either the IP or name of the machine hosting the Podman service
 - `port` defaults to 22
 - `path` defaults to either `/run/podman/podman.sock`, or `/run/user/<uid>/podman/podman.sock` if running rootless.

URL value resolution precedence:
 - command line value
 - environment variable `CONTAINER_HOST`
 - `containers.conf`
 - `unix://run/podman/podman.sock`

#### **--root**=*value*

Storage root dir in which data, including images, is stored (default: "/var/lib/containers/storage" for UID 0, "$HOME/.local/share/containers/storage" for other users).
Default root dir configured in `/etc/containers/storage.conf`.

#### **--runroot**=*value*

Storage state directory where all state information is stored (default: "/var/run/containers/storage" for UID 0, "/var/run/user/$UID/run" for other users).
Default state dir configured in `/etc/containers/storage.conf`.

#### **--runtime**=*value*

Name of the OCI runtime as specified in containers.conf or absolute path to the OCI compatible binary used to run containers.

#### **--runtime-flag**=*flag*

Adds global flags for the container runtime. To list the supported flags, please
consult the manpages of the selected container runtime (`runc` is the default
runtime, the manpage to consult is `runc(8)`.  When the machine is configured
for cgroup V2, the default runtime is `crun`, the manpage to consult is `crun(8)`.).

Note: Do not pass the leading `--` to the flag. To pass the runc flag `--log-format json`
to podman build, the option given would be `--runtime-flag log-format=json`.

#### **--storage-driver**=*value*

Storage driver.  The default storage driver for UID 0 is configured in /etc/containers/storage.conf (`$HOME/.config/containers/storage.conf` in rootless mode), and is *vfs* for non-root users when *fuse-overlayfs* is not available.  The `STORAGE_DRIVER` environment variable overrides the default.  The --storage-driver specified driver overrides all.

Overriding this option will cause the *storage-opt* settings in /etc/containers/storage.conf to be ignored.  The user must
specify additional options via the `--storage-opt` flag.

#### **--storage-opt**=*value*

Storage driver option, Default storage driver options are configured in /etc/containers/storage.conf (`$HOME/.config/containers/storage.conf` in rootless mode). The `STORAGE_OPTS` environment variable overrides the default. The --storage-opt specified options overrides all.

#### **--syslog**=*true|false*

Output logging information to syslog as well as the console (default *false*).

On remote clients, logging is directed to the file $HOME/.config/containers/podman.log.

#### **--tmpdir**

Path to the tmp directory, for libpod runtime content.

NOTE --tmpdir is not used for the temporary storage of downloaded images.  Use the environment variable `TMPDIR` to change the temporary storage location of downloaded container images. Podman defaults to use `/var/tmp`.

#### **--version**, **-v**

Print the version

## Environment Variables

Podman can set up environment variables from env of [engine] table in containers.conf. These variables can be overridden by passing  environment variables before the `podman` commands.

## Remote Access

The Podman command can be used with remote services using the `--remote` flag. Connections can
be made using local unix domain sockets, ssh or directly to tcp sockets. When specifying the
podman --remote flag, only the global options `--url`, `--identity`, `--log-level`, `--connection` are used.

Connection information can also be managed using the containers.conf file.

## Exit Status

The exit code from `podman` gives information about why the container
failed to run or why it exited.  When `podman` commands exit with a non-zero code,
the exit codes follow the `chroot` standard, see below:

  **125** The error is with podman **_itself_**

    $ podman run --foo busybox; echo $?
    Error: unknown flag: --foo
    125

  **126** Executing a _contained command_ and the _command_ cannot be invoked

    $ podman run busybox /etc; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"/etc\": permission denied": OCI runtime error
    126

  **127** Executing a _contained command_ and the _command_ cannot be found
    $ podman run busybox foo; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"foo\": executable file not found in $PATH": OCI runtime error
    127

  **Exit code** _contained command_ exit code

    $ podman run busybox /bin/sh -c 'exit 3'; echo $?
    3


## COMMANDS

<table>
<thead>
<tr>
<th>Command</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><a href="podman-attach.1.md">podman-attach(1)</a></td>
<td>Attach to a running container.</td>
</tr>
<tr>
<td><a href="podman-auto-update.1.md">podman-auto-update(1)</a></td>
<td>Auto update containers according to their auto-update policy</td>
</tr>
<tr>
<td><a href="podman-build.1.md">podman-build(1)</a></td>
<td>Build a container image using a Containerfile.</td>
</tr>
<tr>
<td><a href="podman-commit.1.md">podman-commit(1)</a></td>
<td>Create new image based on the changed container.</td>
</tr>
<tr>
<td><a href="podman-container.1.md">podman-container(1)</a></td>
<td>Manage containers.</td>
</tr>
<tr>
<td><a href="podman-cp.1.md">podman-cp(1)</a></td>
<td>Copy files/folders between a container and the local filesystem.</td>
</tr>
<tr>
<td><a href="podman-create.1.md">podman-create(1)</a></td>
<td>Create a new container.</td>
</tr>
<tr>
<td><a href="podman-diff.1.md">podman-diff(1)</a></td>
<td>Inspect changes on a container or image&#39;s filesystem.</td>
</tr>
<tr>
<td><a href="podman-events.1.md">podman-events(1)</a></td>
<td>Monitor Podman events</td>
</tr>
<tr>
<td><a href="podman-exec.1.md">podman-exec(1)</a></td>
<td>Execute a command in a running container.</td>
</tr>
<tr>
<td><a href="podman-export.1.md">podman-export(1)</a></td>
<td>Export a container&#39;s filesystem contents as a tar archive.</td>
</tr>
<tr>
<td><a href="podman-generate.1.md">podman-generate(1)</a></td>
<td>Generate structured data based for a containers and pods.</td>
</tr>
<tr>
<td><a href="podman-healthcheck.1.md">podman-healthcheck(1)</a></td>
<td>Manage healthchecks for containers</td>
</tr>
<tr>
<td><a href="podman-history.1.md">podman-history(1)</a></td>
<td>Show the history of an image.</td>
</tr>
<tr>
<td><a href="podman-image.1.md">podman-image(1)</a></td>
<td>Manage images.</td>
</tr>
<tr>
<td><a href="podman-images.1.md">podman-images(1)</a></td>
<td>List images in local storage.</td>
</tr>
<tr>
<td><a href="podman-import.1.md">podman-import(1)</a></td>
<td>Import a tarball and save it as a filesystem image.</td>
</tr>
<tr>
<td><a href="podman-info.1.md">podman-info(1)</a></td>
<td>Displays Podman related system information.</td>
</tr>
<tr>
<td><a href="podman-init.1.md">podman-init(1)</a></td>
<td>Initialize one or more containers</td>
</tr>
<tr>
<td><a href="podman-inspect.1.md">podman-inspect(1)</a></td>
<td>Display a container, image, volume, network, or pod&#39;s configuration.</td>
</tr>
<tr>
<td><a href="podman-kill.1.md">podman-kill(1)</a></td>
<td>Kill the main process in one or more containers.</td>
</tr>
<tr>
<td><a href="podman-load.1.md">podman-load(1)</a></td>
<td>Load an image from a container image archive into container storage.</td>
</tr>
<tr>
<td><a href="podman-login.1.md">podman-login(1)</a></td>
<td>Login to a container registry.</td>
</tr>
<tr>
<td><a href="podman-logout.1.md">podman-logout(1)</a></td>
<td>Logout of a container registry.</td>
</tr>
<tr>
<td><a href="podman-logs.1.md">podman-logs(1)</a></td>
<td>Display the logs of one or more containers.</td>
</tr>
<tr>
<td><a href="podman-manifest.1.md">podman-manifest(1)</a></td>
<td>Create and manipulate manifest lists and image indexes.</td>
</tr>
<tr>
<td><a href="podman-mount.1.md">podman-mount(1)</a></td>
<td>Mount a working container&#39;s root filesystem.</td>
</tr>
<tr>
<td><a href="podman-network.1.md">podman-network(1)</a></td>
<td>Manage Podman CNI networks.</td>
</tr>
<tr>
<td><a href="podman-pause.1.md">podman-pause(1)</a></td>
<td>Pause one or more containers.</td>
</tr>
<tr>
<td><a href="podman-play.1.md">podman-play(1)</a></td>
<td>Play pods and containers based on a structured input file.</td>
</tr>
<tr>
<td><a href="podman-pod.1.md">podman-pod(1)</a></td>
<td>Management tool for groups of containers, called pods.</td>
</tr>
<tr>
<td><a href="podman-port.1.md">podman-port(1)</a></td>
<td>List port mappings for a container.</td>
</tr>
<tr>
<td><a href="podman-ps.1.md">podman-ps(1)</a></td>
<td>Prints out information about containers.</td>
</tr>
<tr>
<td><a href="podman-pull.1.md">podman-pull(1)</a></td>
<td>Pull an image from a registry.</td>
</tr>
<tr>
<td><a href="podman-push.1.md">podman-push(1)</a></td>
<td>Push an image from local storage to elsewhere.</td>
</tr>
<tr>
<td><a href="podman-restart.1.md">podman-restart(1)</a></td>
<td>Restart one or more containers.</td>
</tr>
<tr>
<td><a href="podman-rm.1.md">podman-rm(1)</a></td>
<td>Remove one or more containers.</td>
</tr>
<tr>
<td><a href="podman-rmi.1.md">podman-rmi(1)</a></td>
<td>Removes one or more locally stored images.</td>
</tr>
<tr>
<td><a href="podman-run.1.md">podman-run(1)</a></td>
<td>Run a command in a new container.</td>
</tr>
<tr>
<td><a href="podman-save.1.md">podman-save(1)</a></td>
<td>Save an image to a container archive.</td>
</tr>
<tr>
<td><a href="podman-search.1.md">podman-search(1)</a></td>
<td>Search a registry for an image.</td>
</tr>
<tr>
<td><a href="podman-start.1.md">podman-start(1)</a></td>
<td>Start one or more containers.</td>
</tr>
<tr>
<td><a href="podman-stats.1.md">podman-stats(1)</a></td>
<td>Display a live stream of one or more container&#39;s resource usage statistics.</td>
</tr>
<tr>
<td><a href="podman-stop.1.md">podman-stop(1)</a></td>
<td>Stop one or more running containers.</td>
</tr>
<tr>
<td><a href="podman-system.1.md">podman-system(1)</a></td>
<td>Manage podman.</td>
</tr>
<tr>
<td><a href="podman-tag.1.md">podman-tag(1)</a></td>
<td>Add an additional name to a local image.</td>
</tr>
<tr>
<td><a href="podman-top.1.md">podman-top(1)</a></td>
<td>Display the running processes of a container.</td>
</tr>
<tr>
<td><a href="podman-unmount.1.md">podman-unmount(1)</a></td>
<td>Unmount a working container&#39;s root filesystem.</td>
</tr>
<tr>
<td><a href="podman-unpause.1.md">podman-unpause(1)</a></td>
<td>Unpause one or more containers.</td>
</tr>
<tr>
<td><a href="podman-unshare.1.md">podman-unshare(1)</a></td>
<td>Run a command inside of a modified user namespace.</td>
</tr>
<tr>
<td><a href="podman-untag.1.md">podman-untag(1)</a></td>
<td>Removes one or more names from a locally-stored image.</td>
</tr>
<tr>
<td><a href="podman-version.1.md">podman-version(1)</a></td>
<td>Display the Podman version information.</td>
</tr>
<tr>
<td><a href="podman-volume.1.md">podman-volume(1)</a></td>
<td>Simple management tool for volumes.</td>
</tr>
<tr>
<td><a href="podman-wait.1.md">podman-wait(1)</a></td>
<td>Wait on one or more containers to stop and print their exit codes.</td>
</tr>
</tbody>
</table>

## FILES

**containers.conf** (`/usr/share/containers/containers.conf`, `/etc/containers/containers.conf`, `$HOME/.config/containers/containers.conf`)

    Podman has builtin defaults for command line options. These defaults can be overridden using the containers.conf configuration files.

Distributions ship the `/usr/share/containers/containers.conf` file with their default settings. Administrators can override fields in this file by creating the `/etc/containers/containers.conf` file.  Users can further modify defaults by creating the `$HOME/.config/containers/containers.conf` file. Podman merges its builtin defaults with the specified fields from these files, if they exist. Fields specified in the users file override the administrator's file, which overrides the distribution's file, which override the built-in defaults.

Podman uses builtin defaults if no containers.conf file is found.

**mounts.conf** (`/usr/share/containers/mounts.conf`)

    The mounts.conf file specifies volume mount directories that are automatically mounted inside containers when executing the `podman run` or `podman start` commands. Administrators can override the defaults file by creating `/etc/containers/mounts.conf`.

When Podman runs in rootless mode, the file `$HOME/.config/containers/mounts.conf` will override the default if it exists. Please refer to containers-mounts.conf(5) for further details.

**policy.json** (`/etc/containers/policy.json`)

    Signature verification policy files are used to specify policy, e.g. trusted keys, applicable when deciding whether to accept an image, or individual signatures of that image, as valid.

**registries.conf** (`/etc/containers/registries.conf`, `$HOME/.config/containers/registries.conf`)

    registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

    Non root users of Podman can create the `$HOME/.config/containers/registries.conf` file to be used instead of the system defaults.

**storage.conf** (`/etc/containers/storage.conf`, `$HOME/.config/contaners/storage.conf`)

    storage.conf is the storage configuration file for all tools using containers/storage

    The storage configuration file specifies all of the available container storage options for tools using shared container storage.

    When Podman runs in rootless mode, the file `$HOME/.config/containers/storage.conf` is used instead of the system defaults.

## Rootless mode
Podman can also be used as non-root user.  When podman runs in rootless mode, a user namespace is automatically created for the user, defined in /etc/subuid and /etc/subgid.

Containers created by a non-root user are not visible to other users and are not seen or managed by Podman running as root.

It is required to have multiple uids/gids set for an user.  Be sure the user is present in the files `/etc/subuid` and `/etc/subgid`.

If you have a recent version of usermod, you can execute the following
commands to add the ranges to the files

	$ sudo usermod --add-subuids 10000-75535 USERNAME
	$ sudo usermod --add-subgids 10000-75535 USERNAME

Or just add the content manually.

	$ echo USERNAME:10000:65536 >> /etc/subuid
	$ echo USERNAME:10000:65536 >> /etc/subgid

See the `subuid(5)` and `subgid(5)` man pages for more information.

Images are pulled under `XDG_DATA_HOME` when specified, otherwise in the home directory of the user under `.local/share/containers/storage`.

Currently the slirp4netns package is required to be installed to create a network device, otherwise rootless containers need to run in the network namespace of the host.

### **NOTE:** Unsupported file systems in rootless mode

The Overlay file system (OverlayFS) is not supported in rootless mode.  The fuse-overlayfs package is a tool that provides the functionality of OverlayFS in user namespace that allows mounting file systems in rootless environments.  It is recommended to install the fuse-overlayfs package and to enable it by adding `mount_program = "/usr/bin/fuse-overlayfs"` under `[storage.options]` in the `$HOME/.config/containers/storage.conf` file.

The Network File System (NFS) and other distributed file systems (for example: Lustre, Spectrum Scale, the General Parallel File System (GPFS)) are not supported when running in rootless mode as these file systems do not understand user namespace.  However, rootless Podman can make use of an NFS Homedir by modifying the `$HOME/.config/containers/storage.conf` to have the `graphroot` option point to a directory stored on local (Non NFS) storage.

For more information, please refer to the [Podman Troubleshooting Page](https://github.com/containers/podman/blob/master/troubleshooting.md).

## SEE ALSO
`containers-mounts.conf(5)`, `containers-registries.conf(5)`, `containers-storage.conf(5)`, `buildah(1)`, `containers.conf(5)`, `oci-hooks(5)`, `containers-policy.json(5)`, `crun(8)`, `runc(8)`, `subuid(5)`, `subgid(5)`, `slirp4netns(1)`

## HISTORY
Dec 2016, Originally compiled by Dan Walsh <dwalsh@redhat.com>
