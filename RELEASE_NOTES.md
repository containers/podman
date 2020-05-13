# Release Notes

## 1.9.2
### Bugfixes
- Fixed a bug where `podman save` would fail when the target image was specified by digest ([#5234](https://github.com/containers/libpod/issues/5234))
- Fixed a bug where rootless containers with ports forwarded to them could panic and dump core due to a concurrency issue ([#6018](https://github.com/containers/libpod/issues/6018))
- Fixed a bug where rootless Podman could race when opening the rootless user namespace, resulting in commands failing to run
- Fixed a bug where HTTP proxy environment variables forwarded into the container by the `--http-proxy` flag could not be overridden by `--env` or `--env-file` ([#6017](https://github.com/containers/libpod/issues/6017))
- Fixed a bug where rootless Podman was setting resource limits on cgroups v2 systems that were not using systemd-managed cgroups (and thus did not support resource limits), resulting in containers failing to start

### Misc
- Rootless containers will now automatically set their ulimits to the maximum allowed for the user running the container, to match the behavior of containers run as root
- Packages managed by the core Podman team will no longer include a default `libpod.conf`, instead defaulting to `containers.conf`. The default libpod.conf will remain available in the Github repository until the release of Podman 2.0
- The default Podman CNI network configuration now sets HairpinMode to allow containers to access other containers via ports published on the host
- Updated containers/common to v0.8.4

## 1.9.1
### Bugfixes
- Fixed a bug where healthchecks could become nonfunctional if container log paths were manually set with `--log-path` and multiple container logs were placed in the same directory ([#5915](https://github.com/containers/libpod/issues/5915))
- Fixed a bug where rootless Podman could, when using an older `libpod.conf`, print numerous warning messages about an invalid CGroup manager config
- Fixed a bug where rootless Podman would sometimes fail to close the rootless user namespace when joining it ([#5873](https://github.com/containers/libpod/issues/5873))

### Misc
- Updated containers/common to v0.8.2

## 1.9.0
### Features
- Experimental support has been added for `podman run --userns=auto`, which automatically allocates a unique UID and GID range for the new container's user namespace
- The `podman play kube` command now has a `--network` flag to place the created pod in one or more CNI networks
- The `podman commit` command now supports an `--iidfile` flag to write the ID of the committed image to a file
- Initial support for the new `containers.conf` configuration file has been added. `containers.conf` allows for much more detailed configuration of some Podman functionality

### Changes
- There has been a major cleanup of the `podman info` command resulting in breaking changes. Many fields have been renamed to better suit usage with APIv2
- All uses of the `--timeout` flag have been switched to prefer the alternative `--time`. The `--timeout` flag will continue to work, but man pages and `--help` will use the `--time` flag instead

### Bugfixes
- Fixed a bug where some volume mounts from the host would sometimes not properly determine the flags they should use when mounting
- Fixed a bug where Podman was not propagating `$PATH` to Conmon and the OCI runtime, causing issues for some OCI runtimes that required it
- Fixed a bug where rootless Podman would print error messages about missing support for systemd cgroups when run in a container with no cgroup support ([#5488](https://github.com/containers/libpod/issues/5488))
- Fixed a bug where `podman play kube` would not properly handle container-only port mappings ([#5610](https://github.com/containers/libpod/issues/5610))
- Fixed a bug where the `podman container prune` command was not pruning containers in the `created` and `configured` states
- Fixed a bug where Podman was not properly removing CNI IP address allocations after a reboot ([#5433](https://github.com/containers/libpod/issues/5433))
- Fixed a bug where Podman was not properly applying the default Seccomp profile when `--security-opt` was not given at the command line

### HTTP API
- Many Libpod API endpoints have been added, including `Changes`, `Checkpoint`, `Init`, and `Restore`
- Resolved issues where the `podman system service` command would time out and exit while there were still active connections
- Stability overall has greatly improved as we prepare the API for a beta release soon with Podman 2.0

### Misc
- The default infra image for pods has been upgraded to `k8s.gcr.io/pause:3.2` (from 3.1) to address a bug in the architecture metadata for non-AMD64 images
- The `slirp4netns` networking utility in rootless Podman now uses Seccomp filtering where available for improved security
- Updated Buildah to v1.14.8
- Updated containers/storage to v1.18.2
- Updated containers/image to v5.4.3
- Updated containers/common to v0.8.1

## 1.8.2
### Features
- Initial support for automatically updating containers managed via Systemd unit files has been merged. This allows containers to automatically upgrade if a newer version of their image becomes available

### Bugfixes
- Fixed a bug where unit files generated by `podman generate systemd --new` would not force containers to detach, causing the unit to time out when trying to start
- Fixed a bug where `podman system reset` could delete important system directories if run as rootless on installations created by older Podman ([#4831](https://github.com/containers/libpod/issues/4831))
- Fixed a bug where image built by `podman build` would not properly set the OS and Architecture they were built with ([#5503](https://github.com/containers/libpod/issues/5503))
- Fixed a bug where attached `podman run` with `--sig-proxy` enabled (the default), when built with Go 1.14, would repeatedly send signal 23 to the process in the container and could generate errors when the container stopped ([#5483](https://github.com/containers/libpod/issues/5483))
- Fixed a bug where rootless `podman run` commands could hang when forwarding ports
- Fixed a bug where rootless Podman would not work when `/proc` was mounted with the `hidepid` option set
- Fixed a bug where the `podman system service` command would use large amounts of CPU when `--timeout` was set to 0 ([#5531](https://github.com/containers/libpod/issues/5531))

### HTTP API
- Initial support for Libpod endpoints related to creating and operating on image manifest lists has been added
- The Libpod Healthcheck and Events API endpoints are now supported
- The Swagger endpoint can now handle cases where no Swagger documentation has been generated

### Misc
- Updated Buildah to v1.14.3
- Updated containers/storage to v1.16.5
- Several performance improvements have been made to creating containers, which should somewhat improve the performance of `podman create` and `podman run`

## 1.8.1
### Features
- Many networking-related flags have been added to `podman pod create` to enable customization of pod networks, including `--add-host`, `--dns`, `--dns-opt`, `--dns-search`, `--ip`, `--mac-address`, `--network`, and `--no-hosts`
- The `podman ps --format=json` command now includes the ID of the image containers were created with
- The `podman run` and `podman create` commands now feature an `--rmi` flag to remove the image the container was using after it exits (if no other containers are using said image) ([#4628](https://github.com/containers/libpod/issues/4628))
- The `podman create` and `podman run` commands now support the `--device-cgroup-rule` flag ([#4876](https://github.com/containers/libpod/issues/4876))
- While the HTTP API remains in alpha, many fixes and additions have landed. These are documented in a separate subsection below
- The `podman create` and `podman run` commands now feature a `--no-healthcheck` flag to disable healthchecks for a container ([#5299](https://github.com/containers/libpod/issues/5299))
- Containers now recognize the `io.containers.capabilities` label, which specifies a list of capabilities required by the image to run. These capabilities will be used as long as they are more restrictive than the default capabilities used
- YAML produced by the `podman generate kube` command now includes SELinux configuration passed into the container via `--security-opt label=...` ([#4950](https://github.com/containers/libpod/issues/4950))

### Bugfixes
- Fixed CVE-2020-1726, a security issue where volumes manually populated before first being mounted into a container could have those contents overwritten on first being mounted into a container
- Fixed a bug where Podman containers with user namespaces in CNI networks with the DNS plugin enabled would not have the DNS plugin's nameserver added to their `resolv.conf` ([#5256](https://github.com/containers/libpod/issues/5256))
- Fixed a bug where trailing `/` characters in image volume definitions could cause them to not be overridden by a user-specified mount at the same location ([#5219](https://github.com/containers/libpod/issues/5219))
- Fixed a bug where the `label` option in `libpod.conf`, used to disable SELinux by default, was not being respected ([#5087](https://github.com/containers/libpod/issues/5087))
- Fixed a bug where the `podman login` and `podman logout` commands required the registry to log into be specified ([#5146](https://github.com/containers/libpod/issues/5146))
- Fixed a bug where detached rootless Podman containers could not forward ports ([#5167](https://github.com/containers/libpod/issues/5167))
- Fixed a bug where rootless Podman could fail to run if the pause process had died
- Fixed a bug where Podman ignored labels that were specified with only a key and no value ([#3854](https://github.com/containers/libpod/issues/3854))
- Fixed a bug where Podman would fail to create named volumes when the backing filesystem did not support SELinux labelling ([#5200](https://github.com/containers/libpod/issues/5200))
- Fixed a bug where `--detach-keys=""` would not disable detaching from a container ([#5166](https://github.com/containers/libpod/issues/5166))
- Fixed a bug where the `podman ps` command was too aggressive when filtering containers and would force `--all` on in too many situations
- Fixed a bug where the `podman play kube` command was ignoring image configuration, including volumes, working directory, labels, and stop signal ([#5174](https://github.com/containers/libpod/issues/5174))
- Fixed a bug where the `Created` and `CreatedTime` fields in `podman images --format=json` were misnamed, which also broke Go template output for those fields ([#5110](https://github.com/containers/libpod/issues/5110))
- Fixed a bug where rootless Podman containers with ports forwarded could hang when started ([#5182](https://github.com/containers/libpod/issues/5182))
- Fixed a bug where `podman pull` could fail to parse registry names including port numbers
- Fixed a bug where Podman would incorrectly attempt to validate image OS and architecture when starting containers
- Fixed a bug where Bash completion for `podman build -f` would not list available files that could be built ([#3878](https://github.com/containers/libpod/issues/3878))
- Fixed a bug where `podman commit --change` would perform incorrect validation, resulting in valid changes being rejected ([#5148](https://github.com/containers/libpod/issues/5148))
- Fixed a bug where `podman logs --tail` could take large amounts of memory when the log file for a container was large ([#5131](https://github.com/containers/libpod/issues/5131))
- Fixed a bug where Podman would sometimes incorrectly generate firewall rules on systems using `firewalld`
- Fixed a bug where the `podman inspect` command would not display network information for containers properly if a container joined multiple CNI networks ([#4907](https://github.com/containers/libpod/issues/4907))
- Fixed a bug where the `--uts` flag to `podman create` and `podman run` would only allow specifying containers by full ID ([#5289](https://github.com/containers/libpod/issues/5289))
- Fixed a bug where rootless Podman could segfault when passed a large number of file descriptors
- Fixed a bug where the `podman port` command was incorrectly interpreting additional arguments as container names, instead of port numbers
- Fixed a bug where units created by `podman generate systemd` did not depend on network targets, and so could start before the system network was ready ([#4130](https://github.com/containers/libpod/issues/4130))
- Fixed a bug where exec sessions in containers which did not specify a user would not inherit supplemental groups added to the container via `--group-add`
- Fixed a bug where Podman would not respect the `$TMPDIR` environment variable for placing large temporary files during some operations (e.g. `podman pull`) ([#5411](https://github.com/containers/libpod/issues/5411))

### HTTP API
- Initial support for secure connections to servers via SSH tunneling has been added
- Initial support for the libpod `create` and `logs` endpoints for containers has been added
- Added a `/swagger/` endpoint to serve API documentation
- The `json` endpoint for containers has received many fixes
- Filtering images and containers has been greatly improved, with many bugs fixed and documentation improved
- Image creation endpoints (commit, pull, etc) have seen many fixes
- Server timeout has been fixed so that long operations will no longer trigger the timeout and shut the server down
- The `stats` endpoint for containers has seen major fixes and now provides accurate output
- Handling the HTTP 304 status code has been fixed for all endpoints
- Many fixes have been made to API documentation to ensure it matches the code

### Misc
- Updated vendored Buildah to v1.14.2
- Updated vendored containers/storage to v1.16.2
- The `Created` field to `podman images --format=json` has been renamed to `CreatedSince` as part of the fix for ([#5110](https://github.com/containers/libpod/issues/5110)). Go templates using the old name should still work
- The `CreatedTime` field to `podman images --format=json` has been renamed to `CreatedAt` as part of the fix for ([#5110](https://github.com/containers/libpod/issues/5110)). Go templates using the old name should still work
- The `before` filter to `podman images` has been renamed to `since` for Docker compatibility. Using `before` will still work, but documentation has been changed to use the new `since` filter
- Using the `--password` flag to `podman login` now warns that passwords are being passed in plaintext
- Some common cases where Podman would deadlock have been fixed to warn the user that `podman system renumber` must be run to resolve the deadlock

## 1.8.0
### Features
- The `podman system service` command has been added, providing a preview of Podman's new Docker-compatible API. This API is still very new, and not yet ready for production use, but is available for early testing
- Rootless Podman now uses Rootlesskit for port forwarding, which should greatly improve performance and capabilities
- The `podman untag` command has been added to remove tags from images without deleting them
- The `podman inspect` command on images now displays previous names they used
- The `podman generate systemd` command now supports a `--new` option to generate service files that create and run new containers instead of managing existing containers
- Support for `--log-opt tag=` to set logging tags has been added to the `journald` log driver
- Added support for using Seccomp profiles embedded in images for `podman run` and `podman create` via the new `--seccomp-policy` CLI flag ([#4806](https://github.com/containers/libpod/pull/4806))
- The `podman play kube` command now honors pull policy ([#4880](https://github.com/containers/libpod/issues/4880))

### Bugfixes
- Fixed a bug where the `podman cp` command would not copy the contents of directories when paths ending in `/.` were given ([#4717](https://github.com/containers/libpod/issues/4717))
- Fixed a bug where the `podman play kube` command did not properly locate Seccomp profiles specified relative to localhost ([#4555](https://github.com/containers/libpod/issues/4555))
- Fixed a bug where the `podman info` command for remote Podman did not show registry information ([#4793](https://github.com/containers/libpod/issues/4793))
- Fixed a bug where the `podman exec` command did not support having input piped into it ([#3302](https://github.com/containers/libpod/issues/3302))
- Fixed a bug where the `podman cp` command with rootless Podman on CGroups v2 systems did not properly determine if the container could be paused while copying ([#4813](https://github.com/containers/libpod/issues/4813))
- Fixed a bug where the `podman container prune --force` command could possible remove running containers if they were started while the command was running ([#4844](https://github.com/containers/libpod/issues/4844))
- Fixed a bug where Podman, when run as root, would not properly configure `slirp4netns` networking when requested ([#4853](https://github.com/containers/libpod/pull/4853))
- Fixed a bug where `podman run --userns=keep-id` did not work when the user had a UID over 65535 ([#4838](https://github.com/containers/libpod/issues/4838))
- Fixed a bug where rootless `podman run` and `podman create` with the `--userns=keep-id` option could change permissions on `/run/user/$UID` and break KDE ([#4846](https://github.com/containers/libpod/issues/4846))
- Fixed a bug where rootless Podman could not be run in a systemd service on systems using CGroups v2 ([#4833](https://github.com/containers/libpod/issues/4833))
- Fixed a bug where `podman inspect` would show CPUShares as 0, instead of the default (1024), when it was not explicitly set ([#4822](https://github.com/containers/libpod/issues/4822))
- Fixed a bug where `podman-remote push` would segfault ([#4706](https://github.com/containers/libpod/issues/4706))
- Fixed a bug where image healthchecks were not shown in the output of `podman inspect` ([#4799](https://github.com/containers/libpod/issues/4799))
- Fixed a bug where named volumes created with containers from pre-1.6.3 releases of Podman would be autoremoved with their containers if the `--rm` flag was given, even if they were given names ([#5009](https://github.com/containers/libpod/issues/5009))
- Fixed a bug where `podman history` was not computing image sizes correctly ([#4916](https://github.com/containers/libpod/issues/4916))
- Fixed a bug where Podman would not error on invalid values to the `--sort` flag to `podman images`
- Fixed a bug where providing a name for the image made by `podman commit` was mandatory, not optional as it should be ([#5027](https://github.com/containers/libpod/issues/5027))
- Fixed a bug where the remote Podman client would append an extra `"` to `%PATH` ([#4335](https://github.com/containers/libpod/issues/4335))
- Fixed a bug where the `podman build` command would sometimes ignore the `-f` option and build the wrong Containerfile
- Fixed a bug where the `podman ps --filter` command would only filter running containers, instead of all containers, if `--all` was not passed ([#5050](https://github.com/containers/libpod/issues/5050))
- Fixed a bug where the `podman load` command on compressed images would leave an extra copy on disk
- Fixed a bug where the `podman restart` command would not properly clean up the network, causing it to function differently from `podman stop; podman start` ([#5051](https://github.com/containers/libpod/issues/5051))
- Fixed a bug where setting the `--memory-swap` flag to `podman create` and `podman run` to `-1` (to indicate unlimited) was not supported ([#5091](https://github.com/containers/libpod/issues/5091))

### Misc
- Initial work on version 2 of the Podman remote API has been merged, but is still in an alpha state and not ready for use. Read more [here](https://podman.io/releases/2020/01/17/podman-new-api.html)
- Many formatting corrections have been made to the manpages
- The changes to address ([#5009](https://github.com/containers/libpod/issues/5009)) may cause anonymous volumes created by Podman versions 1.6.3 to 1.7.0 to not be removed when their container is removed
- Updated vendored Buildah to v1.13.1
- Updated vendored containers/storage to v1.15.8
- Updated vendored containers/image to v5.2.0

## 1.7.0
### Features
- Added support for setting a static MAC address for containers
- Added support for creating `macvlan` networks with `podman network create`, allowing Podman containers to be attached directly to networks the host is connected to
- The `podman image prune` and `podman container prune` commands now support the `--filter` flag to filter what will be pruned, and now prompts for confirmation when run without `--force` ([#4410](https://github.com/containers/libpod/issues/4410) and [#4411](https://github.com/containers/libpod/issues/4411))
- Podman now creates CGroup namespaces by default on systems using CGroups v2 ([#4363](https://github.com/containers/libpod/issues/4363))
- Added the `podman system reset` command to remove all Podman files and perform a factory reset of the Podman installation
- Added the `--history` flag to `podman images` to display previous names used by images ([#4566](https://github.com/containers/libpod/issues/4566))
- Added the `--ignore` flag to `podman rm` and `podman stop` to not error when requested containers no longer exist
- Added the `--cidfile` flag to `podman rm` and `podman stop` to read the IDs of containers to be removed or stopped from a file
- The `podman play kube` command now honors Seccomp annotations ([#3111](https://github.com/containers/libpod/issues/3111))
- The `podman play kube` command now honors `RunAsUser`, `RunAsGroup`, and `selinuxOptions`
- The output format of the `podman version` command has been changed to better match `docker version` when using the `--format` flag
- Rootless Podman will no longer initialize containers/storage twice, removing a potential deadlock preventing Podman commands from running while an image was being pulled ([#4591](https://github.com/containers/libpod/issues/4591))
- Added `tmpcopyup` and `notmpcopyup` options to the `--tmpfs` and `--mount type=tmpfs` flags to `podman create` and `podman run` to control whether the content of directories are copied into tmpfs filesystems mounted over them
- Added support for disabling detaching from containers by setting empty detach keys via `--detach-keys=""`
- The `podman build` command now supports the `--pull` and `--pull-never` flags to control when images are pulled during a build
- The `podman ps -p` command now shows the name of the pod as well as its ID ([#4703](https://github.com/containers/libpod/issues/4703))
- The `podman inspect` command on containers will now display the command used to create the container
- The `podman info` command now displays information on registry mirrors ([#4553](https://github.com/containers/libpod/issues/4553))

### Bugfixes
- Fixed a bug where Podman would use an incorrect runtime directory as root, causing state to be deleted after root logged out and making Podman in systemd services not function properly
- Fixed a bug where the `--change` flag to `podman import` and `podman commit` was not being parsed properly in many cases
- Fixed a bug where detach keys specified in `libpod.conf` were not used by the `podman attach` and `podman exec` commands, which always used the global default `ctrl-p,ctrl-q` key combination ([#4556](https://github.com/containers/libpod/issues/4556))
- Fixed a bug where rootless Podman was not able to run `podman pod stats` even on CGroups v2 enabled systems ([#4634](https://github.com/containers/libpod/issues/4634))
- Fixed a bug where rootless Podman would fail on kernels without the `renameat2` syscall ([#4570](https://github.com/containers/libpod/issues/4570))
- Fixed a bug where containers with chained network namespace dependencies (IE, container A using `--net container=B` and container B using `--net container=C`) would not properly mount `/etc/hosts` and `/etc/resolv.conf` into the container ([#4626](https://github.com/containers/libpod/issues/4626))
- Fixed a bug where `podman run` with the `--rm` flag and without `-d` could, when run in the background, throw a 'container does not exist' error when attempting to remove the container after it exited
- Fixed a bug where named volume locks were not properly reacquired after a reboot, potentially leading to deadlocks when trying to start containers using the volume ([#4605](https://github.com/containers/libpod/issues/4605) and [#4621](https://github.com/containers/libpod/issues/4621))
- Fixed a bug where Podman could not completely remove containers if sent SIGKILL during removal, leaving the container name unusable without the `podman rm --storage` command to complete removal ([#3906](https://github.com/containers/libpod/issues/3906))
- Fixed a bug where checkpointing containers started with `--rm` was allowed when `--export` was not specified (the container, and checkpoint, would be removed after checkpointing was complete by `--rm`) ([#3774](https://github.com/containers/libpod/issues/3774))
- Fixed a bug where the `podman pod prune` command would fail if containers were present in the pods and the `--force` flag was not passed ([#4346](https://github.com/containers/libpod/issues/4346))
- Fixed a bug where containers could not set a static IP or static MAC address if they joined a non-default CNI network ([#4500](https://github.com/containers/libpod/issues/4500))
- Fixed a bug where `podman system renumber` would always throw an error if a container was mounted when it was run
- Fixed a bug where `podman container restore` would fail with containers using a user namespace
- Fixed a bug where rootless Podman would attempt to use the journald events backend even on systems without systemd installed
- Fixed a bug where `podman history` would sometimes not properly identify the IDs of layers in an image ([#3359](https://github.com/containers/libpod/issues/3359))
- Fixed a bug where containers could not be restarted when Conmon v2.0.3 or later was used
- Fixed a bug where Podman did not check image OS and Architecture against the host when starting a container
- Fixed a bug where containers in pods did not function properly with the Kata OCI runtime ([#4353](https://github.com/containers/libpod/issues/4353))
- Fixed a bug where `podman info --format '{{ json . }}' would not produce JSON output ([#4391](https://github.com/containers/libpod/issues/4391))
- Fixed a bug where Podman would not verify if files passed to `--authfile` existed ([#4328](https://github.com/containers/libpod/issues/4328))
- Fixed a bug where `podman images --digest` would not always print digests when they were available
- Fixed a bug where rootless `podman run` could hang due to a race with reading and writing events
- Fixed a bug where rootless Podman would print warning-level logs despite not be instructed to do so ([#4456](https://github.com/containers/libpod/issues/4456))
- Fixed a bug where `podman pull` would attempt to fetch from remote registries when pulling an unqualified image using the `docker-daemon` transport ([#4434](https://github.com/containers/libpod/issues/4434))
- Fixed a bug where `podman cp` would not work if STDIN was a pipe
- Fixed a bug where `podman exec` could stop accepting input if anything was typed between the command being run and the exec session starting ([#4397](https://github.com/containers/libpod/issues/4397))
- Fixed a bug where `podman logs --tail 0` would print all lines of a container's logs, instead of no lines ([#4396](https://github.com/containers/libpod/issues/4396))
- Fixed a bug where the timeout for `slirp4netns` was incorrectly set, resulting in an extremely long timeout ([#4344](https://github.com/containers/libpod/issues/4344))
- Fixed a bug where the `podman stats` command would print CPU utilizations figures incorrectly ([#4409](https://github.com/containers/libpod/issues/4409))
- Fixed a bug where the `podman inspect --size` command would not print the size of the container's read/write layer if the size was 0 ([#4744](https://github.com/containers/libpod/issues/4744))
- Fixed a bug where the `podman kill` command was not properly validating signals before use ([#4746](https://github.com/containers/libpod/issues/4746))
- Fixed a bug where the `--quiet` and `--format` flags to `podman ps` could not be used at the same time
- Fixed a bug where the `podman stop` command was not stopping exec sessions when a container was created without a PID namespace (`--pid=host`)
- Fixed a bug where the `podman pod rm --force` command was not removing anonymous volumes for containers that were removed
- Fixed a bug where the `podman checkpoint` command would not export all changes to the root filesystem of the container if performed more than once on the same container ([#4606](https://github.com/containers/libpod/issues/4606))
- Fixed a bug where containers started with `--rm` would not be automatically removed on being stopped if an exec session was running inside the container ([#4666](https://github.com/containers/libpod/issues/4666))

### Misc
- The fixes to runtime directory path as root can cause strange behavior if an upgrade is performed while containers are running
- Updated vendored Buildah to v1.12.0
- Updated vendored containers/storage library to v1.15.4
- Updated vendored containers/image library to v5.1.0
- Kata Containers runtimes (`kata-runtime`, `kata-qemu`, and `kata-fc`) are now present in the default libpod.conf, but will not be available unless Kata containers is installed on the system
- Podman previously did not allow the creation of containers with a memory limit lower than 4MB. This restriction has been removed, as the `crun` runtime can create containers with significantly less memory

## 1.6.3
### Features
- Handling of the `libpod.conf` configuration file has seen major changes. Most significantly, rootless users will no longer automatically receive a complete configuration file when they first use Podman, and will instead only receive differences from the global configuration.
- Initial support for the CNI DNS plugin, which allows containers to resolve the IPs of other containers via DNS name, has been added
- Podman now supports anonymous named volumes, created by specifying only a destination to the `-v` flag to the `podman create` and `podman run` commands
- Named volumes now support `uid` and `gid` options in `--opt o=...` to set UID and GID of the created volume

### Bugfixes
- Fixed a bug where the `podman start` command would print container ID, instead of name, when starting containers given their name
- Fixed a bug where named volumes with options did not properly detect issues with mounting the volume, leading to an inconsistent state ([#4303](https://github.com/containers/libpod/issues/4303))
- Fixed a bug where incorrect Seccomp profiles were used in containers generated by `podman play kube`
- Fixed a bug where processes started by `podman exec` would have the wrong SELinux label in some circumstances ([#4361](https://github.com/containers/libpod/issues/4361))
- Fixed a bug where error messages from `slirp4netns` would be lost
- Fixed a bug where `podman run --network=$NAME` would not throw an error in rootless Podman, where CNI networks are not supported
- Fixed a bug where `podman network create` would throw confusing errors when trying to create a volume with a name that already exists
- Fixed a bug where Podman would not error if the `systemd` CGroup manager was specified, but systemd could not be contacted over DBus
- Fixed a bug where image volumes were mounted `noexec` ([#4318](https://github.com/containers/libpod/issues/4318))
- Fixed a bug where the `podman stats` command required the name of a container to be given, instead of showing all containers when no container was specified ([#4274](https://github.com/containers/libpod/issues/4274))
- Fixed a bug where the `podman volume inspect` command would not show the options that named volumes were created with
- Fixed a bug where custom storage configuration was not written to `storage.conf` at time of first creation for rootless Podman ([#2659](https://github.com/containers/libpod/issues/2659))
- Fixed a bug where remote Podman did not support shell redirection of container output

### Misc
- Updated vendored containers/image library to v5.0
- Initial support for images using manifest lists has been added, though commands for directly interacting with manifests are still missing
- Support for pushing to and pulling from OSTree has been removed due to deprecation in the containers/image library
- Rootless Podman no longer enables linger on systems with systemd as init by default. As such, containers will now be killed when the user who ran them logs out, unless linger is explicitly enabled using [loginctl](https://www.freedesktop.org/software/systemd/man/loginctl.html)
- Podman will now check the version of `conmon` that is in use to ensure it is sufficient

## 1.6.2
### Features
- Added a `--runtime` flag to `podman system migrate` to allow the OCI runtime for all containers to be reset, to ease transition to the `crun` runtime on CGroups V2 systems until `runc` gains full support
- The `podman rm` command can now remove containers in broken states which previously could not be removed
- The `podman info` command, when run without root, now shows information on UID and GID mappings in the rootless user namespace
- Added `podman build --squash-all` flag, which squashes all layers (including those of the base image) into one layer
- The `--systemd` flag to `podman run` and `podman create` now accepts a string argument and allows a new value, `always`, which forces systemd support without checking if the the container entrypoint is systemd

### Bugfixes
- Fixed a bug where the `podman top` command did not work on systems using CGroups V2 ([#4192](https://github.com/containers/libpod/issues/4192))
- Fixed a bug where rootless Podman could double-close a file, leading to a panic
- Fixed a bug where rootless Podman could fail to retrieve some containers while refreshing the state
- Fixed a bug where `podman start --attach --sig-proxy=false` would still proxy signals into the container
- Fixed a bug where Podman would unconditionally use a non-default path for authentication credentials (`auth.json`), breaking `podman login` integration with `skopeo` and other tools using the containers/image library
- Fixed a bug where `podman ps --format=json` and `podman images --format=json` would display `null` when no results were returned, instead of valid JSON
- Fixed a bug where `podman build --squash` was incorrectly squashing all layers into one, instead of only new layers
- Fixed a bug where rootless Podman would allow volumes with options to be mounted (mounting volumes requires root), creating an inconsistent state where volumes reported as mounted but were not ([#4248](https://github.com/containers/libpod/issues/4248))
- Fixed a bug where volumes which failed to unmount could not be removed ([#4247](https://github.com/containers/libpod/issues/4247))
- Fixed a bug where Podman incorrectly handled some errors relating to unmounted or missing containers in containers/storage
- Fixed a bug where `podman stats` was broken on systems running CGroups V2 when run rootless ([#4268](https://github.com/containers/libpod/issues/4268))
- Fixed a bug where the `podman start` command would print the short container ID, instead of the full ID
- Fixed a bug where containers created with an OCI runtime that is no longer available (uninstalled or removed from the config file) would not appear in `podman ps` and could not be removed via `podman rm`
- Fixed a bug where containers restored via `podman container restore --import` would retain the CGroup path of the original container, even if their container ID changed; thus, multiple containers created from the same checkpoint would all share the same CGroup

### Misc
- The default PID limit for containers is now set to 4096. It can be adjusted back to the old default (unlimited) by passing `--pids-limit 0` to `podman create` and `podman run`
- The `podman start --attach` command now automatically attaches `STDIN` if the container was created with `-i`
- The `podman network create` command now validates network names using the same regular expression as container and pod names
- The `--systemd` flag to `podman run` and `podman create` will now only enable systemd mode when the binary being run inside the container is `/sbin/init`, `/usr/sbin/init`, or ends in `systemd` (previously detected any path ending in `init` or `systemd`)
- Updated vendored Buildah to 1.11.3
- Updated vendored containers/storage to 1.13.5
- Updated vendored containers/image to 4.0.1

## 1.6.1
### Bugfixes
- Fixed a bug where rootless Podman on systems using CGroups V2 would not function with the `cgroupfs` CGroups manager
- Fixed a bug where rootless Podman could not correctly identify the DBus session address, causing containers to fail to start ([#4162](https://github.com/containers/libpod/issues/4162))
- Fixed a bug where rootless Podman with `slirp4netns` networking would fail to start containers due to mount leaks

## 1.6.0
### Features
- The `podman network create`, `podman network rm`, `podman network inspect`, and `podman network ls` commands have been added to manage CNI networks used by Podman
- The `podman volume create` command can now create and mount volumes with options, allowing volumes backed by NFS, tmpfs, and many other filesystems
- Podman can now run containers without CGroups for better integration with systemd by using the `--cgroups=disabled` flag with `podman create` and `podman run`. This is presently only supported with the `crun` OCI runtime
- The `podman volume rm` and `podman volume inspect` commands can now refer to volumes by an unambiguous partial name, in addition to full name (e.g. `podman volume rm myvol` to remove a volume named `myvolume`) ([#3891](https://github.com/containers/libpod/issues/3891))
- The `podman run` and `podman create` commands now support the `--pull` flag to allow forced re-pulling of images ([#3734](https://github.com/containers/libpod/issues/3734))
- Mounting volumes into a container using `--volume`, `--mount`, and `--tmpfs` now allows the `suid`, `dev`, and `exec` mount options (the inverse of `nosuid`, `nodev`, `noexec`) ([#3819](https://github.com/containers/libpod/issues/3819))
- Mounting volumes into a container using `--mount` now allows the `relabel=Z` and `relabel=z` options to relabel mounts.
- The `podman push` command now supports the `--digestfile` option to save a file containing the pushed digest
- Pods can now have their hostname set via `podman pod create --hostname` or providing Pod YAML with a hostname set to `podman play kube` ([#3732](https://github.com/containers/libpod/issues/3732))
- The `podman image sign` command now supports the `--cert-dir` flag
- The `podman run` and `podman create` commands now support the `--security-opt label=filetype:$LABEL` flag to set the SELinux label for container files
- The remote Podman client now supports healthchecks

### Bugfixes
- Fixed a bug where remote `podman pull` would panic if a Varlink connection was not available ([#4013](https://github.com/containers/libpod/issues/4013))
- Fixed a bug where `podman exec` would not properly set terminal size when creating a new exec session ([#3903](https://github.com/containers/libpod/issues/3903))
- Fixed a bug where `podman exec` would not clean up socket symlinks on the host ([#3962](https://github.com/containers/libpod/issues/3962))
- Fixed a bug where Podman could not run systemd in containers that created a CGroup namespace
- Fixed a bug where `podman prune -a` would attempt to prune images used by Buildah and CRI-O, causing errors ([#3983](https://github.com/containers/libpod/issues/3983))
- Fixed a bug where improper permissions on the `~/.config` directory could cause rootless Podman to use an incorrect directory for storing some files
- Fixed a bug where the bash completions for `podman import` threw errors
- Fixed a bug where Podman volumes created with `podman volume create` would not copy the contents of their mountpoint the first time they were mounted into a container ([#3945](https://github.com/containers/libpod/issues/3945))
- Fixed a bug where rootless Podman could not run `podman exec` when the container was not run inside a CGroup owned by the user ([#3937](https://github.com/containers/libpod/issues/3937))
- Fixed a bug where `podman play kube` would panic when given Pod YAML without a `securityContext` ([#3956](https://github.com/containers/libpod/issues/3956))
- Fixed a bug where Podman would place files incorrectly when `storage.conf` configuration items were set to the empty string ([#3952](https://github.com/containers/libpod/issues/3952))
- Fixed a bug where `podman build` did not correctly inherit Podman's CGroup configuration, causing crashed on CGroups V2 systems ([#3938](https://github.com/containers/libpod/issues/3938))
- Fixed a bug where `podman cp` would improperly copy files on the host when copying a symlink in the container that included a glob operator ([#3829](https://github.com/containers/libpod/issues/3829))
- Fixed a bug where remote `podman run --rm` would exit before the container was completely removed, allowing race conditions when removing container resources ([#3870](https://github.com/containers/libpod/issues/3870))
- Fixed a bug where rootless Podman would not properly handle changes to `/etc/subuid` and `/etc/subgid` after a container was launched
- Fixed a bug where rootless Podman could not include some devices in a container using the `--device` flag ([#3905](https://github.com/containers/libpod/issues/3905))
- Fixed a bug where the `commit` Varlink API would segfault if provided incorrect arguments ([#3897](https://github.com/containers/libpod/issues/3897))
- Fixed a bug where temporary files were not properly cleaned up after a build using remote Podman ([#3869](https://github.com/containers/libpod/issues/3869))
- Fixed a bug where `podman remote cp` crashed instead of reporting it was not yet supported ([#3861](https://github.com/containers/libpod/issues/3861))
- Fixed a bug where `podman exec` would run as the wrong user when execing into a container was started from an image with Dockerfile `USER` (or a user specified via `podman run --user`) ([#3838](https://github.com/containers/libpod/issues/3838))
- Fixed a bug where images pulled using the `oci:` transport would be improperly named
- Fixed a bug where `podman varlink` would hang when managed by systemd due to SD_NOTIFY support conflicting with Varlink ([#3572](https://github.com/containers/libpod/issues/3572))
- Fixed a bug where mounts to the same destination would sometimes not trigger a conflict, causing a race as to which was actually mounted
- Fixed a bug where `podman exec --preserve-fds` caused Podman to hang ([#4020](https://github.com/containers/libpod/issues/4020))
- Fixed a bug where removing an unmounted container that was unmounted might sometimes not properly clean up the container ([#4033](https://github.com/containers/libpod/issues/4033))
- Fixed a bug where the Varlink server would freeze when run in a systemd unit file ([#4005](https://github.com/containers/libpod/issues/4005))
- Fixed a bug where Podman would not properly set the `$HOME` environment variable when the OCI runtime did not set it
- Fixed a bug where rootless Podman would incorrectly print warning messages when an OCI runtime was not found ([#4012](https://github.com/containers/libpod/issues/4012))
- Fixed a bug where named volumes would conflict with, instead of overriding, `tmpfs` filesystems added by the `--read-only-tmpfs` flag to `podman create` and `podman run`
- Fixed a bug where `podman cp` would incorrectly make the target directory when copying to a symlink which pointed to a nonexistent directory ([#3894](https://github.com/containers/libpod/issues/3894))
- Fixed a bug where remote Podman would incorrectly read `STDIN` when the `-i` flag was not set ([#4095](https://github.com/containers/libpod/issues/4095))
- Fixed a bug where `podman play kube` would create an empty pod when given an unsupported YAML type ([#4093](https://github.com/containers/libpod/issues/4093))
- Fixed a bug where `podman import --change` improperly parsed `CMD` ([#4000](https://github.com/containers/libpod/issues/4000))

### Misc
- Significant changes were made to Podman volumes in this release. If you have pre-existing volumes, it is strongly recommended to run `podman system renumber` after upgrading.
- Version 0.8.1 or greater of the CNI Plugins is now required for Podman
- Version 2.0.1 or greater of Conmon is strongly recommended
- Updated vendored Buildah to v1.11.2
- Updated vendored containers/storage library to v1.13.4
- Improved error messages when trying to create a pod with no name via `podman play kube`
- Improved error messages when trying to run `podman pause` or `podman stats` on a rootless container on a system without CGroups V2 enabled
- `TMPDIR` has been set to `/var/tmp` by default to better handle large temporary files
- `podman wait` has been optimized to detect stopped containers more rapidly
- Podman containers now include a `ContainerManager` annotation indicating they were created by `libpod`
- The `podman info` command now includes information about `slirp4netns` and `fuse-overlayfs` if they are available
- Podman no longer sets a default size of 65kb for tmpfs filesystems
- The default Podman CNI network has been renamed in an attempt to prevent conflicts with CRI-O when both are run on the same system. This should only take effect on system restart
- The output of `podman volume inspect` has been more closely matched to `docker volume inspect`

## 1.5.1
### Features
- The hostname of pods is now set to the pod's name

### Bugfixes
- Fixed a bug where `podman run` and `podman create` did not honor the `--authfile` option ([#3730](https://github.com/containers/libpod/issues/3730))
- Fixed a bug where containers restored with `podman container restore --import` would incorrectly duplicate the Conmon PID file of the original container
- Fixed a bug where `podman build` ignored the default OCI runtime configured in `libpod.conf`
- Fixed a bug where `podman run --rm` (or force-removing any running container with `podman rm --force`) were not retrieving the correct exit code ([#3795](https://github.com/containers/libpod/issues/3795))
- Fixed a bug where Podman would exit with an error if any configured hooks directory was not present
- Fixed a bug where `podman inspect` and `podman commit` would not use the correct `CMD` for containers run with `podman play kube`
- Fixed a bug created pods when using rootless Podman and CGroups V2 ([#3801](https://github.com/containers/libpod/issues/3801))
- Fixed a bug where the `podman events` command with the `--since` or `--until` options could take a very long time to complete

### Misc
- Rootless Podman will now inherit OCI runtime configuration from the root configuration ([#3781](https://github.com/containers/libpod/issues/3781))
- Podman now properly sets a user agent while contacting registries ([#3788](https://github.com/containers/libpod/issues/3788))

## 1.5.0
### Features
- Podman containers can now join the user namespaces of other containers with `--userns=container:$ID`, or a user namespace at an arbitrary path with `--userns=ns:$PATH`
- Rootless Podman can experimentally squash all UIDs and GIDs in an image to a single UID and GID (which does not require use of the `newuidmap` and `newgidmap` executables) by passing `--storage-opt ignore_chown_errors`
- The `podman generate kube` command now produces YAML for any bind mounts the container has created ([#2303](https://github.com/containers/libpod/issues/2303))
- The `podman container restore` command now features a new flag, `--ignore-static-ip`, that can be used with `--import` to import a single container with a static IP multiple times on the same host
- Added the ability for `podman events` to output JSON by specifying `--format=json`
- If the OCI runtime or `conmon` binary cannot be found at the paths specified in `libpod.conf`, Podman will now also search for them in the calling user's path
- Added the ability to use `podman import` with URLs ([#3609](https://github.com/containers/libpod/issues/3609))
- The `podman ps` command now supports filtering names using regular expressions ([#3394](https://github.com/containers/libpod/issues/3394))
- Rootless Podman containers with `--privileged` set will now mount in all host devices that the user can access
- The `podman create` and `podman run` commands now support the `--env-host` flag to forward all environment variables from the host into the container
- Rootless Podman now supports healthchecks ([#3523](https://github.com/containers/libpod/issues/3523))
- The format of the `HostConfig` portion of the output of `podman inspect` on containers has been improved and synced with Docker
- Podman containers now support CGroup namespaces, and can create them by passing `--cgroupns=private` to `podman run` or `podman create`
- The `podman create` and `podman run` commands now support the `--ulimit=host` flag, which uses any ulimits currently set on the host for the container
- The `podman rm` and `podman rmi` commands now use different exit codes to indicate 'no such container' and 'container is running' errors
- Support for CGroups V2 through the `crun` OCI runtime has been greatly improved, allowing resource limits to be set for rootless containers when the CGroups V2 hierarchy is in use

### Bugfixes
- Fixed a bug where a race condition could cause `podman restart` to fail to start containers with ports
- Fixed a bug where containers restored from a checkpoint would not properly report the time they were started at
- Fixed a bug where `podman search` would return at most 25 results, even when the maximum number of results was set higher
- Fixed a bug where `podman play kube` would not honor capabilities set in imported YAML ([#3689](https://github.com/containers/libpod/issues/3689))
- Fixed a bug where `podman run --env`, when passed a single key (to use the value from the host), would set the environment variable in the container even if it was not set on the host ([#3648](https://github.com/containers/libpod/issues/3648))
- Fixed a bug where `podman commit --changes` would not properly set environment variables
- Fixed a bug where Podman could segfault while working with images with no history
- Fixed a bug where `podman volume rm` could remove arbitrary volumes if given an ambiguous name ([#3635](https://github.com/containers/libpod/issues/3635))
- Fixed a bug where `podman exec` invocations leaked memory by not cleaning up files in tmpfs
- Fixed a bug where the `--dns` and `--net=container` flags to `podman run` and `podman create` were not mutually exclusive ([#3553](https://github.com/containers/libpod/issues/3553))
- Fixed a bug where rootless Podman would be unable to run containers when less than 5 UIDs were available
- Fixed a bug where containers in pods could not be removed without removing the entire pod ([#3556](https://github.com/containers/libpod/issues/3556))
- Fixed a bug where Podman would not properly clean up all CGroup controllers for created cgroups when using the `cgroupfs` CGroup driver
- Fixed a bug where Podman containers did not properly clean up files in tmpfs, resulting in a memory leak as containers stopped
- Fixed a bug where healthchecks from images would not use default settings for interval, retries, timeout, and start period when they were not provided by the image ([#3525](https://github.com/containers/libpod/issues/3525))
- Fixed a bug where healthchecks using the `HEALTHCHECK CMD` format where not properly supported ([#3507](https://github.com/containers/libpod/issues/3507))
- Fixed a bug where volume mounts using relative source paths would not be properly resolved ([#3504](https://github.com/containers/libpod/issues/3504))
- Fixed a bug where `podman run` did not use authorization credentials when a custom path was specified ([#3524](https://github.com/containers/libpod/issues/3524))
- Fixed a bug where containers checkpointed with `podman container checkpoint` did not properly set their finished time
- Fixed a bug where running `podman inspect` on any container not created with `podman run` or `podman create` (for example, pod infra containers) would result in a segfault ([#3500](https://github.com/containers/libpod/issues/3500))
- Fixed a bug where healthcheck flags for `podman create` and `podman run` were incorrectly named ([#3455](https://github.com/containers/libpod/pull/3455))
- Fixed a bug where Podman commands would fail to find targets if a partial ID was specified that was ambiguous between a container and pod ([#3487](https://github.com/containers/libpod/issues/3487))
- Fixed a bug where restored containers would not have the correct SELinux label
- Fixed a bug where Varlink endpoints were not working properly if `more` was not correctly specified
- Fixed a bug where the Varlink PullImage endpoint would crash if an error occurred ([#3715](https://github.com/containers/libpod/issues/3715))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not allow boolean arguments for its `ro` and `rw` options ([#2980](https://github.com/containers/libpod/issues/2980))
- Fixed a bug where pods did not properly share the UTS namespace, resulting in incorrect behavior from some utilities which rely on hostname ([#3547](https://github.com/containers/libpod/issues/3547))
- Fixed a bug where Podman would unconditionally append `ENTRYPOINT` to `CMD` during `podman commit` (and when reporting `CMD` in `podman inspect`) ([#3708](https://github.com/containers/libpod/issues/3708))
- Fixed a bug where `podman events` with the `journald` events backend would incorrectly print 6 previous events when only new events were requested ([#3616](https://github.com/containers/libpod/issues/3616))
- Fixed a bug where `podman port` would exit prematurely when a port number was specified ([#3747](https://github.com/containers/libpod/issues/3747))
- Fixed a bug where passing `.` as an argument to the `--dns-search` flag to `podman create` and `podman run` was not properly clearing DNS search domains in the container

### Misc
- Updated vendored Buildah to v1.10.1
- Updated vendored containers/image to v3.0.2
- Updated vendored containers/storage to v1.13.1
- Podman now requires conmon v2.0.0 or higher
- The `podman info` command now displays the events logger being in use
- The `podman inspect` command on containers now includes the ID of the pod a container has joined and the PID of the container's conmon process
- The `-v` short flag for `podman --version` has been re-added
- Error messages from `podman pull` should be significantly clearer
- The `podman exec` command is now available in the remote client

## 1.4.4
### Bugfixes
- Fixed a bug where rootless Podman would attempt to use the entire root configuration if no rootless configuration was present for the user, breaking rootless Podman for new installations
- Fixed a bug where rootless Podman's pause process would block SIGTERM, preventing graceful system shutdown and hanging until the system's init send SIGKILL
- Fixed a bug where running Podman as root with `sudo -E` would not work after running rootless Podman at least once
- Fixed a bug where options for `tmpfs` volumes added with the `--tmpfs` flag were being ignored
- Fixed a bug where images with no layers could not properly be displayed and removed by Podman
- Fixed a bug where locks were not properly freed on failure to create a container or pod

### Misc
- Updated containers/storage to v1.12.13

## 1.4.3
### Features
- Podman now has greatly improved support for containers using multiple OCI runtimes. Containers now remember if they were created with a different runtime using `--runtime` and will always use that runtime
- The `cached` and `delegated` options for volume mounts are now allowed for Docker compatibility ([#3340](https://github.com/containers/libpod/issues/3340))
- The `podman diff` command now supports the `--latest` flag

### Bugfixes
- Fixed a bug where `podman cp` on a single file would create a directory at the target and place the file in it ([#3384](https://github.com/containers/libpod/issues/3384))
- Fixed a bug where `podman inspect --format '{{.Mounts}}'` would print a hexadecimal address instead of a container's mounts
- Fixed a bug where rootless Podman would not add an entry to container's `/etc/hosts` files for their own hostname ([#3405](https://github.com/containers/libpod/issues/3405))
- Fixed a bug where `podman ps --sync` would segfault ([#3411](https://github.com/containers/libpod/issues/3411))
- Fixed a bug where `podman generate kube` would produce an invalid ports configuration ([#3408](https://github.com/containers/libpod/issues/3408))

### Misc
- Podman now performs much better on systems with heavy I/O load
- The `--cgroup-manager` flag to `podman` now shows the correct default setting in help if the default was overridden by `libpod.conf`
- For backwards compatibility, setting `--log-driver=json-file` in `podman run` is now supported as an alias for `--log-driver=k8s-file`. This is considered deprecated, and `json-file` will be moved to a new implementation in the future ([#3363](https://github.com/containers/libpod/issues/3363))
- Podman's default `libpod.conf` file now allows the [crun](https://github.com/giuseppe/crun) OCI runtime to be used if it is installed

## 1.4.2
### Bugfixes
- Fixed a bug where Podman could not run containers using an older version of Systemd as init ([#3295](https://github.com/containers/libpod/issues/3295))

### Misc
- Updated vendored Buildah to v1.9.0 to resolve a critical bug with Dockerfile `RUN` instructions
- The error message for running `podman kill` on containers that are not running has been improved
- The Podman remote client can now log to a file if syslog is not available

## 1.4.1
### Features
- The `podman exec` command now sets its error code differently based on whether the container does not exist, and the command in the container does not exist
- The `podman inspect` command on containers now outputs Mounts JSON that matches that of `docker inspect`, only including user-specified volumes and differentiating bind mounts and named volumes
- The `podman inspect` command now reports the path to a container's OCI spec with the `OCIConfigPath` key (only included when the container is initialized or running)
- The `podman run --mount` command now supports the `bind-nonrecursive` option for bind mounts ([#3314](https://github.com/containers/libpod/issues/3314))

### Bugfixes
- Fixed a bug where `podman play kube` would fail to create containers due to an unspecified log driver
- Fixed a bug where Podman would fail to build with [musl libc](https://www.musl-libc.org/) ([#3284](https://github.com/containers/libpod/issues/3284))
- Fixed a bug where rootless Podman using `slirp4netns` networking in an environment with no nameservers on the host other than localhost would result in nonfunctional networking ([#3277](https://github.com/containers/libpod/issues/3277))
- Fixed a bug where `podman import` would not properly set environment variables, discarding their values and retaining only keys
- Fixed a bug where Podman would fail to run when built with Apparmor support but run on systems without the Apparmor kernel module loaded ([#3331](https://github.com/containers/libpod/issues/3331))

### Misc
- Remote Podman will now default the username it uses to log in to remote systems to the username of the current user
- Podman now uses JSON logging with OCI runtimes that support it, allowing for better error reporting
- Updated vendored Buildah to v1.8.4
- Updated vendored containers/image to v2.0

## 1.4.0
### Features
- The `podman checkpoint` and `podman restore` commands can now be used to migrate containers between Podman installations on different systems ([#1618](https://github.com/containers/libpod/issues/1618))
- The `podman cp` command now supports a `pause` flag to pause containers while copying into them
- The remote client now supports a configuration file for pre-configuring connections to remote Podman installations

### Bugfixes
- Fixed CVE-2019-10152 - The `podman cp` command improperly dereferenced symlinks in host context
- Fixed a bug where `podman commit` could improperly set environment variables that contained `=` characters ([#3132](https://github.com/containers/libpod/issues/3132))
- Fixed a bug where rootless Podman would sometimes fail to start containers with forwarded ports ([#2942](https://github.com/containers/libpod/issues/2942))
- Fixed a bug where `podman version` on the remote client could segfault ([#3145](https://github.com/containers/libpod/issues/3145))
- Fixed a bug where `podman container runlabel` would use `/proc/self/exe` instead of the path of the Podman command when printing the command being executed
- Fixed a bug where filtering images by label did not work ([#3163](https://github.com/containers/libpod/issues/3163))
- Fixed a bug where specifying a bing mount or tmpfs mount over an image volume would cause a container to be unable to start ([#3174](https://github.com/containers/libpod/issues/3174))
- Fixed a bug where `podman generate kube` did not work with containers with named volumes
- Fixed a bug where rootless Podman would receive `permission denied` errors accessing `conmon.pid` ([#3187](https://github.com/containers/libpod/issues/3187))
- Fixed a bug where `podman cp` with a folder specified as target would replace the folder, as opposed to copying into it ([#3184](https://github.com/containers/libpod/issues/3184))
- Fixed a bug where rootless Podman commands could double-unlock a lock, causing a crash ([#3207](https://github.com/containers/libpod/issues/3207))
- Fixed a bug where Podman incorrectly set `tmpcopyup` on `/dev/` mounts, causing errors when using the Kata containers runtime ([#3229](https://github.com/containers/libpod/issues/3229))
- Fixed a bug where `podman exec` would fail on older kernels ([#2968](https://github.com/containers/libpod/issues/2968))

### Misc
- The `podman inspect` command on containers now uses the `Id` key (instead of `ID`) for the container's ID, for better compatibility with the output of `docker inspect`
- The `podman commit` command is now usable with the Podman remote client
- The `--signature-policy` flag (used with several image-related commands) has been deprecated
- The `podman unshare` command now defines two environment variables in the spawned shell: `CONTAINERS_RUNROOT` and `CONTAINERS_GRAPHROOT`, pointing to temporary and permanent storage for rootless containers
- Updated vendored containers/storage and containers/image libraries with numerous bugfixes
- Updated vendored Buildah to v1.8.3
- Podman now requires [Conmon v0.2.0](https://github.com/containers/conmon/releases/tag/v0.2.0)
- The `podman cp` command is now aliased as `podman container cp`
- Rootless Podman will now default `init_path` using root Podman's configuration files (`/etc/containers/libpod.conf` and `/usr/share/containers/libpod.conf`) if not overridden in the rootless configuration

## 1.3.1
### Features
- The `podman cp` command can now read input redirected to `STDIN`, and output to `STDOUT` instead of a file, using `-` instead of an argument.
- The Podman remote client now displays version information from both the client and server in `podman version`
- The `podman unshare` command has been added, allowing easy entry into the user namespace set up by rootless Podman (allowing the removal of files created by rootless Podman, among other things)

### Bugfixes
- Fixed a bug where Podman containers with the `--rm` flag were removing created volumes when they were automatically removed ([#3071](https://github.com/containers/libpod/issues/3071))
- Fixed a bug where container and pod locks were incorrectly marked as released after a system reboot, causing errors on container and pod removal ([#2900](https://github.com/containers/libpod/issues/2900))
- Fixed a bug where Podman pods could not be removed if any container in the pod encountered an error during removal ([#3088](https://github.com/containers/libpod/issues/3088))
- Fixed a bug where Podman pods run with the `cgroupfs` CGroup driver would encounter a race condition during removal, potentially failing to remove the pod CGroup
- Fixed a bug where the `podman container checkpoint` and `podman container restore` commands were not visible in the remote client
- Fixed a bug where `podman remote ps --ns` would not print the container's namespaces ([#2938](https://github.com/containers/libpod/issues/2938))
- Fixed a bug where removing stopped containers with healthchecks could cause an error
- Fixed a bug where the default `libpod.conf` file was causing parsing errors ([#3095](https://github.com/containers/libpod/issues/3095))
- Fixed a bug where pod locks were not being freed when pods were removed, potentially leading to lock exhaustion
- Fixed a bug where 'podman run' with SD_NOTIFY set could, on short-running containers, create an inconsistent state rendering the container unusable

### Misc
- The remote Podman client now uses the Varlink bridge to establish remote connections by default

## 1.3.0
### Features
- Podman now supports container restart policies! The `--restart` flag on `podman create` and `podman run` allows containers to be restarted after they exit. Please note that Podman cannot restart containers after a system reboot - for that, see our next feature
- Podman `podman generate systemd` command was added to generate systemd unit files for managing Podman containers
- The `podman runlabel` command now allows a `$GLOBAL_OPTS` variable, which will be populated by global options passed to the `podman runlabel` command, allowing custom storage configurations to be passed into containers run with `runlabel` ([#2399](https://github.com/containers/libpod/issues/2399))
- The `podman play kube` command now allows `File` and `FileOrCreate` volumes
- The `podman pod prune` command was added to prune unused pods
- Added the `podman system migrate` command to migrate containers using older configurations to allow their use by newer Libpod versions ([#2935](https://github.com/containers/libpod/issues/2935))
- Podman containers now forward proxy-related environment variables from the host into the container with the `--http-proxy` flag (enabled by default)
- Read-only Podman containers can now create tmpfs filesystems on `/tmp`, `/var/tmp`, and `/run` with the `--read-only-tmpfs` flag (enabled by default)
- The `podman init` command was added, performing all container pre-start tasks without starting the container to allow pre-run debugging

### Bugfixes
- Fixed a bug where `podman cp` would not copy folders ([#2836](https://github.com/containers/libpod/issues/2836))
- Fixed a bug where Podman would panic when the Varlink API attempted too pull a non-existent image ([#2860](https://github.com/containers/libpod/issues/2860))
- Fixed a bug where `podman rmi` sometimes did not produce an event when images were deleted
- Fixed a bug where Podman would panic when the Varlink API passed improperly-formatted options when attempting to build ([#2869](https://github.com/containers/libpod/issues/2869))
- Fixed a bug where `podman images` would not print a header if no images were present ([#2877](https://github.com/containers/libpod/pull/2877))
- Fixed a bug where the `podman images` command with `--filter dangling=false` would incorrectly print dangling images instead of images which are not dangling ([#2884](https://github.com/containers/libpod/issues/2884))
- Fixed a bug where rootless Podman would panic when any command was run after the system was rebooted ([#2894](https://github.com/containers/libpod/issues/2894))
- Fixed a bug where Podman containers in user namespaces would include undesired directories from the host in `/sys/kernel`
- Fixed a bug where `podman create` would panic when trying to create a container whose name already existed
- Fixed a bug where `podman pull` would exit 0 on failing to pull an image ([#2785](https://github.com/containers/libpod/issues/2785))
- Fixed a bug where `podman pull` would not properly print the cause of errors that occurred ([#2710](https://github.com/containers/libpod/issues/2710))
- Fixed a bug where rootless Podman commands were not properly suspended via `ctrl-z` in a shell ([#2775](https://github.com/containers/libpod/issues/2775))
- Fixed a bug where Podman would error when cleaning up containers when some container mountpoints in `/sys/` were cleaned up already by the closing of the mount namespace
- Fixed a bug where `podman play kube` was not including environment variables from the image run ([#2930](https://github.com/containers/libpod/issues/2930))
- Fixed a bug where `podman play kube` would not properly clean up partially-created pods when encountering an error
- Fixed a bug where `podman commit` with the `--change` flag improperly set `CMD` when a multipart value was provided ([#2951](https://github.com/containers/libpod/issues/2951))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not properly validate its arguments, causing Podman to panic
- Fixed a bug where conflicts between mounts created by the `--mount`, `--volume`, and `--tmpfs` flags were not properly reported
- Fixed a bug where the `--mount` flag could not be used with named volumes
- Fixed a bug where the `--mount` flag did not properly set options for created tmpfs filesystems
- Fixed a bug where rootless Podman could close too many file descriptors, causing Podman to panic ([#2964](https://github.com/containers/libpod/issues/2964))
- Fixed a bug where `podman logout` would not print an error when the login was established by `docker login` ([#2735](https://github.com/containers/libpod/issues/2735))
- Fixed a bug where `podman stop` would error when not all containers were running ([#2993](https://github.com/containers/libpod/issues/2993))
- Fixed a bug where `podman pull` would fail to pull images by shortname if they were not present in the `docker.io` registry
- Fixed a bug where `podman login` would error when credentials were not present if a credential helper was configured ([#1675](https://github.com/containers/libpod/issues/1675))
- Fixed a bug where the `podman system renumber` command and Podman post-reboot state refreshes would not create events
- Fixed a bug where the `podman top` command was not compatible with `docker top` syntax

### Misc
- Updated vendored Buildah to v1.8.2
- Updated vendored containers/storage to v1.12.6
- Updated vendored containers/psgo to v1.2.1
- Updated to sysregistriesv2, including slight changes to the `registries.conf` config file
- Rootless Podman now places all containers within a single user namespace. This change will not take effect for existing containers until containers are restarted, and containers that are not restarted may not be fully usable
- The `podman run`, `podman create`, `podman start`, `podman restart`, `podman attach`, `podman stop`, `podman port`, `podman rm`, `podman top`, `podman image tree`, `podman generate kube`, `podman umount`, `podman container checkpoint`, and `podman container restore` commands are now available in the remote client
- The Podman remote client now builds on Windows
- A major refactor of volumes created using the `podman volume` command was performed. There should be no major user-facing changes, but downgrading from Podman 1.3 to previous versions may render some volumes unable to be removed.
- The `podman events` command now logs events to journald by default. The old behavior (log to file) can be configured in podman.conf via the `events_logger` option
- The `podman commit` command, in versions 1.2 and earlier, included all volumes mounted into the container as image volumes in the committed image. This behavior was incorrect and has been disabled by default; it can be re-enabled with the `--include-volumes` flag


## 1.2.0
### Features
- Podman now supports image healthchecks! The `podman healthcheck run` command was added to manually run healthchecks, and the status of a running healthcheck can be viewed via `podman inspect`
- The `podman events` command was added to show a stream of significant events
- The `podman ps` command now supports a `--watch` flag that will refresh its output on a given interval
- The `podman image tree` command was added to show a tree representation of an image's layers
- The `podman logs` command can now display logs for multiple containers at the same time ([#2219](https://github.com/containers/libpod/issues/2219))
- The `podman exec` command can now pass file descriptors to the process being executed in the container via the `--preserve-fds` option ([#2372](https://github.com/containers/libpod/issues/2372))
- The `podman images` command can now filter images by reference ([#2266](https://github.com/containers/libpod/issues/2266))
- The `podman system df` command was added to show disk usage by Podman
- The `--add-host` option can now be used by containers sharing a network namespace ([#2504](https://github.com/containers/libpod/issues/2504))
- The `podman cp` command now has an `--extract` option to extract the contents of a Tar archive and copy them into the container, instead of copying the archive itself ([#2520](https://github.com/containers/libpod/issues/2520))
- Podman now allows manually specifying the path of the `slirp4netns` binary for rootless networking via the `--network-cmd-path` flag ([#2506](https://github.com/containers/libpod/issues/2506))
- Rootless Podman can now be used with a single UID and GID, without requiring a full 65536 UIDs/GIDs to be allocated in `/etc/subuid` and `/etc/subgid` ([#1651](https://github.com/containers/libpod/issues/1651))
- The `podman runlabel` command now supports the `--replace` option to replace containers using the name requested
- Infrastructure containers for Podman pods will now attempt to use the image's `CMD` and `ENTRYPOINT` instead of a fixed command ([#2182](https://github.com/containers/libpod/issues/2182))
- The `podman play kube` command now supports the `HostPath` and `VolumeMounts` YAML fields ([#2536](https://github.com/containers/libpod/issues/2536))
- Added support to disable creation of `resolv.conf` or `/etc/hosts` in containers by specifying `--dns=none` and `--no-hosts`, respectively, to `podman run` and `podman create` ([#2744](https://github.com/containers/libpod/issues/2744))
- The `podman version` command now supports the `{{ json . }}` template (which outputs JSON)
- Podman can now forward ports using the SCTP protocol

### Bugfixes
- Fixed a bug where directories could not be passed to `podman run --device` ([#2380](https://github.com/containers/libpod/issues/2380))
- Fixed a bug where rootless Podman with the `--config` flag specified would not use appropriate defaults ([#2510](https://github.com/containers/libpod/issues/2510))
- Fixed a bug where rootless Podman containers using the host network (`--net=host`) would show SELinux as enabled in the container when there were no privileges to use it
- Fixed a bug where importing very large images from `STDIN` could cause Podman to run out of memory
- Fixed a bug where some images would fail to run due to symlinks in paths where Podman would normally mount tmpfs filesystems
- Fixed a bug where `podman play kube` would sometimes segfault ([#2209](https://github.com/containers/libpod/issues/2209))
- Fixed a bug where `podman runlabel` did not respect the `$PWD` variable ([#2171](https://github.com/containers/libpod/issues/2171))
- Fixed a bug where error messages from refreshing the state in rootless Podman were not properly displayed ([#2584](https://github.com/containers/libpod/issues/2584))
- Fixed a bug where rootless `podman build` could not access DNS servers when `slirp4netns` was in use ([#2572](https://github.com/containers/libpod/issues/2572))
- Fixed a bug where rootless `podman stop` and `podman rm` would not work on containers which specified a non-root user ([#2577](https://github.com/containers/libpod/issues/2577))
- Fixed a bug where container labels whose values contained commas were incorrectly parsed and caused errors creating containers ([#2574](https://github.com/containers/libpod/issues/2574))
- Fixed a bug where calling Podman with a nonexistent command would exit 0, instead of with an appropriate error code ([#2530](https://github.com/containers/libpod/issues/2530))
- Fixed a bug where rootless `podman exec` would fail when `--user` was specified ([#2566](https://github.com/containers/libpod/issues/2566))
- Fixed a bug where, when a container had a name that was a fragment of another container's ID, Podman would refuse to operate on the first container by name
- Fixed a bug where `podman pod create` would fail if a pod shared no namespaces but created an infra container
- Fixed a bug where rootless Podman failed on the S390 and CRIS architectures
- Fixed a bug where `podman rm` would exit 0 if no containers specified were found ([#2539](https://github.com/containers/libpod/issues/2539))
- Fixed a bug where `podman run` would fail to enable networking for containers with additional CNI networks specified ([#2795](https://github.com/containers/libpod/issues/2795))
- Fixed a bug where the `podman images` command on the remote client was not displaying digests ([#2756](https://github.com/containers/libpod/issues/2756))
- Fixed a bug where Podman was unable to clean up mounts in containers using user namespaces
- Fixed a bug where `podman image save` would, when told to save to a path that exists, return an error, but still delete the file at the given path
- Fixed a bug where specifying environment variables containing commas with `--env` would cause parsing errors ([#2712](https://github.com/containers/libpod/issues/2712))
- Fixed a bug where `podman umount` would not error if called with no arguments
- Fixed a bug where the user and environment variables specified by the image used in containers created by `podman create kube` was being ignored ([#2665](https://github.com/containers/libpod/issues/2665))
- Fixed a bug where the `podman pod inspect` command would segfault if not given an argument ([#2681](https://github.com/containers/libpod/issues/2681))
- Fixed a bug where rootless `podman pod top` would fail ([#2682](https://github.com/containers/libpod/issues/2682))
- Fixed a bug where the `podman load` command would not error if an input file is not specified and a file was not redirected to `STDIN`
- Fixed a bug where rootless `podman` could fail if global configuration was altered via flag (for example, `--root`, `--runroot`, `--storage-driver`)
- Fixed a bug where forwarded ports that were part of a range (e.g. 20-30) were displayed individually by `podman ps`, as opposed to together as a range ([#1358](https://github.com/containers/libpod/issues/1358))
- Fixed a bug where `podman run --rootfs` could panic ([#2654](https://github.com/containers/libpod/issues/2654))
- Fixed a bug where `podman build` would fail if options were specified after the directory to build ([#2636](https://github.com/containers/libpod/issues/2636))
- Fixed a bug where image volumes made by `podman create` and `podman run` would have incorrect permissions ([#2634](https://github.com/containers/libpod/issues/2634))
- Fixed a bug where rootless containers were not using the containers/image blob cache, leading to slower image pulls
- Fixed a bug where the `podman image inspect` command incorrectly allowed the `--latest`, `--type`, and `--size` options

### Misc
- Updated Buildah to v1.7.2
- Updated `psgo` library to v1.2, featuring greatly improved safety during concurrent use
- The `podman events` command may not show all activity regarding images, as only Podman was instrumented; images created, deleted, or pulled by CRI-O or Buildah will not be shown in `podman events`
- The `podman pod top` and `podman pod stats` commands are now usable with the Podman remote client
- The `podman kill` and `podman wait` commands are now usable with the Podman remote client
- Removed the unused `restarting` state and mapped `stopped` (also unused) to `exited` in `podman ps --filter status`
- Podman container, pod, and volume names may now contain the `.` (period) character

## 1.1.2
### Bugfixes
- Fixed a bug where the `podman image list`, `podman image rm`, and `podman container list` had broken global storage options
- Fixed a bug where the `--label` option to `podman create` and `podman run` was missing the `-l` alias
- Fixed a bug where running Podman with the `--config` flag would not set an appropriate default value for `tmp_dir` ([#2408](https://github.com/containers/libpod/issues/2408))
- Fixed a bug where the `podman logs` command with the `--timestamps` flag produced unreadable output ([#2500](https://github.com/containers/libpod/issues/2500))
- Fixed a bug where the `podman cp` command would automatically extract `.tar` files copied into the container ([#2509](https://github.com/containers/libpod/issues/2509))

### Misc
- The `podman container stop` command is now usable with the Podman remote client

## 1.1.1
### Bugfixes
- Fixed a bug where `podman container restore` was erroneously available as `podman restore` ([#2191](https://github.com/containers/libpod/issues/2191))
- Fixed a bug where the `volume_path` option in `libpod.conf` was not being respected
- Fixed a bug where Podman failed to build when the `varlink` tag was not present ([#2459](https://github.com/containers/libpod/issues/2459))
- Fixed a bug where the `podman image load` command was listed twice in help text
- Fixed a bug where the `podman image sign` command was also listed as `podman sign`
- Fixed a bug where the `podman image list` command incorrectly had an `image` alias
- Fixed a bug where the `podman images` command incorrectly had `ls` and `list` aliases
- Fixed a bug where the `podman image rm` command was being displayed as `podman image rmi`
- Fixed a bug where the `podman create` command would attempt to parse arguments meant for the container
- Fixed a bug where the combination of FIPS mode and user namespaces resulted in permissions errors
- Fixed a bug where the `--time` alias for `--timeout` for the `podman restart` and `podman stop` commands did not function
- Fixed a bug where the default stop timeout for newly-created containers was being set to 0 seconds (resulting in an immediate SIGKILL on running `podman stop`)
- Fixed a bug where the output format of `podman port` was incorrect, printing full container ID instead of truncated ID
- Fixed a bug where the `podman container list` command did not exist
- Fixed a bug where `podman build` could not build a container from images tagged locally that did not exist in a registry ([#2469](https://github.com/containers/libpod/issues/2469))
- Fixed a bug where some Podman commands that accept no arguments would not error when provided arguments
- Fixed a bug where `podman play kube` could not handle cases where a pod and a container shared a name

### Misc
- Usage text for many commands was greatly improved
- Major cleanups were made to Podman manpages, ensuring that command lists are accurate
- Greatly improved debugging output when the `newuidmap` and `newgidmap` binaries fail when using rootless Podman
- The `-s` alias for the global `--storage-driver` option has been removed
- The `podman container refresh` command has been deprecated, as its intended use case is no longer relevant. The command has been hidden and manpages deleted. It will be removed in a future release
- The `podman container runlabel` command will now pull images not available locally even without the `--pull` option. The `--pull` option has been deprecated
- The `podman container checkpoint` and `podman container restore` commands are now only available on OCI runtimes where they are supported (e.g. `runc`)

## 1.1.0
### Features
- Added `--latest` and `--all` flags to `podman mount` and `podman umount`
- Rootless Podman can now forward ports into containers (using the same `-p` and `-P` flags as root Podman)
- Rootless Podman will now pull some configuration options (for example, OCI runtime path) from the default root `libpod.conf` if they are not explicitly set in the user's own `libpod.conf` ([#2174](https://github.com/containers/libpod/issues/2174))
- Added an alias `-f` for the `--format` flag of the `podman info` and `podman version` commands
- Added an alias `-s` for the `--size` flag of the `podman inspect` command
- Added the `podman system info` and `podman system prune` commands
- Added the `podman cp` command to copy files between containers and the host ([#613](https://github.com/containers/libpod/issues/613))
- Added the `--password-stdin` flag to `podman login`
- Added the `--all-tags` flag to `podman pull`
- The `--rm` and `--detach` flags can now be used together with `podman run`
- The `podman start` and `podman run` commands for containers in pods will now start dependency containers if they are stopped
- Added the `podman system renumber` command to handle lock changes
- The `--net=host` and `--dns` flags for `podman run` and `podman create` no longer conflict
- Podman now handles mounting the shared /etc/resolv.conf from network namespaces created by `ip netns add` when they are passed in via `podman run --net=ns:`

### Bugfixes
- Fixed a bug with `podman inspect` where different information would be returned when the container was running versus when it was stopped
- Fixed a bug where errors in Go templates passed to `podman inspect` were silently ignored instead of reported to the user ([#2159](https://github.com/containers/libpod/issues/2159))
- Fixed a bug where rootless Podman with `--pid=host` containers was incorrectly masking paths in `/proc`
- Fixed a bug where full errors starting rootless `Podman` were not reported when a refresh was requested
- Fixed a bug where Podman would override the config file-specified storage driver with the driver the backing database was created with without warning users
- Fixed a bug where `podman prune` would prune all images not in use by a container, as opposed to only untagged images, by default ([#2192](https://github.com/containers/libpod/issues/2192))
- Fixed a bug where `podman create --quiet` and `podman run --quiet` were not properly suppressing output
- Fixed a bug where the `table` keyword in Go template output of `podman ps` was not working ([#2221](https://github.com/containers/libpod/issues/2221))
- Fixed a bug where `podman inspect` on images pulled by digest would double-print `@sha256` in output when printing digests ([#2086](https://github.com/containers/libpod/issues/2086))
- Fixed a bug where `podman container runlabel` will return a non-0 exit code if the label does not exist
- Fixed a bug where container state was always reset to Created after a reboot ([#1703](https://github.com/containers/libpod/issues/1703))
- Fixed a bug where `/dev/pts` was unconditionally overridden in rootless Podman, which was unnecessary except in very specific cases
- Fixed a bug where Podman run as root was ignoring some options in `/etc/containers/storage.conf` ([#2217](https://github.com/containers/libpod/issues/2217))
- Fixed a bug where Podman cleanup processes were not being given the proper OCI runtime path if a custom one was specified
- Fixed a bug where `podman images --filter dangling=true` would crash if no dangling images were present ([#2246](https://github.com/containers/libpod/issues/2246))
- Fixed a bug where `podman ps --format "{{.Mounts}}"` would not display a container's mounts ([#2238](https://github.com/containers/libpod/issues/2238))
- Fixed a bug where `podman pod stats` was ignoring Go templates specified by `--format` ([#2258](https://github.com/containers/libpod/issues/2258))
- Fixed a bug where `podman generate kube` would fail on containers with `--user` specified ([#2304](https://github.com/containers/libpod/issues/2304))
- Fixed a bug where `podman images` displayed incorrect output for images pulled by digest ([#2175](https://github.com/containers/libpod/issues/2175))
- Fixed a bug where `podman port` and `podman ps` did not properly display ports if the container joined a network namespace from a pod or another container ([#846](https://github.com/containers/libpod/issues/846))
- Fixed a bug where detaching from a container using the detach keys would cause Podman to hang until the container exited
- Fixed a bug where `podman create --rm` did not work with `podman start --attach`
- Fixed a bug where invalid named volumes specified in `podman create` and `podman run` could cause segfaults ([#2301](https://github.com/containers/libpod/issues/2301))
- Fixed a bug where the `runtime` field in `libpod.conf` was being ignored. `runtime` is legacy and deprecated, but will continue to be respected for the foreseeable future
- Fixed a bug where `podman login` would sometimes report it logged in successfully when it did not
- Fixed a bug where `podman pod create` would not error on receiving unused CLI argument
- Fixed a bug where rootless `podman run` with the `--pod` argument would fail if the pod was stopped
- Fixed a bug where `podman images` did not print a trailing newline when not invoked on a TTY ([#2388](https://github.com/containers/libpod/issues/2388))
- Fixed a bug where the `--runtime` option was sometimes not overriding `libpod.conf`
- Fixed a bug where `podman pull` and `podman runlabel` would sometimes exit with 0 when they should have exited with an error ([#2405](https://github.com/containers/libpod/issues/2405))
- Fixed a bug where rootless `podman export -o` would fail ([#2381](https://github.com/containers/libpod/issues/2381))
- Fixed a bug where read-only volumes would fail in rootless Podman when the volume originated on a filesystem mounted `nosuid`, `nodev`, or `noexec` ([#2312](https://github.com/containers/libpod/issues/2312))
- Fixed a bug where some files used by checkpoint and restore received improper SELinux labels ([#2334](https://github.com/containers/libpod/issues/2334))
- Fixed a bug where Podman's volume path was not properly changed when containers/storage changed location ([#2395](https://github.com/containers/libpod/issues/2395))

### Misc
- Podman migrated to a new, shared memory locking model in this release. As part of this, if you are running Podman with pods or dependency containers (e.g. `--net=container:`), you should run the `podman system renumber` command to migrate your containers to the new model - please reference the `podman-system-renumber(1)` man page for further details
- Podman migrated to a new command-line parsing library, and the output format of help and usage text has somewhat changed as a result
- Updated Buildah to v1.7, picking up a number of bugfixes
- Updated containers/image library to v1.5, picking up a number of bugfixes and performance improvements to pushing images
- Updated containers/storage library to v1.10, picking up a number of bugfixes
- Work on the remote Podman client for interacting with Podman remotely over Varlink is progressing steadily, and many image and pod commands are supported - please see the [Readme](https://github.com/containers/libpod/blob/master/remote_client.md) for details
- Added path masking to mounts with the `:z` and `:Z` options, preventing users from accidentally performing an SELinux relabel of their entire home directory
- The `podman container runlabel` command will not pull an image if it does not contain the requested label
- Many commands' usage information now includes examples
- `podman rm` can now delete containers in containers/storage, which can be used to resolve some situations where Podman fails to remove a container
- The `podman search` command now searches multiple registries in parallel for improved performance
- The `podman build` command now defaults `--pull-always` to true
- Containers which share a network namespace (for example, when in a pod) will now share /etc/hosts and /etc/resolv.conf between all containers in the pod, causing changes in one container to propagate to all containers sharing their networks
- The `podman rm` and `podman rmi` commands now return 1 (instead of 127) when all specified container or images are missing

## 1.0.0
### Features
- The `podman exec` command now includes a `--workdir` option to set working directory for the executed command
- The `podman create` and `podman run` commands now support the `--init` flag to use a minimal init process in the container
- Added the `podman image sign` command to GPG sign images
- The `podman run --device` flag now accepts directories, and will added any device nodes in the directory to the container
- Added the `podman play kube` command to create pods and containers from Kubernetes pod YAML

### Bugfixes
- Fixed a bug where passing `podman create` or `podman run` volumes with an empty host or container path could cause a segfault
- Fixed a bug where `storage.conf` was sometimes ignored for rootless containers
- Fixed a bug where Podman run as root would error if CAP_SYS_RESOURCE was not available
- Fixed a bug where Podman would fail to start containers after a system restart due to an out-of-date default Apparmor profile
- Fixed a bug where Podman's bash completions were not working
- Fixed a bug where `podman login` would use existing login credentials even if new credentials were provided
- Fixed a bug where Podman could create some directories with the wrong permissions, breaking containers with user namespaces
- Fixed a bug where `podman runlabel` was not properly setting container names when the `--name` was specified
- Fixed a bug where `podman runlabel` sometimes included extra spaces in command output
- Fixed a bug where `podman commit` was including invalid port numbers in created images when committing containers with published ports
- Fixed a bug where `podman exec` was not honoring the container's environment variables
- Fixed a bug where `podman run --device` would fail when a symlink to a device was specified
- Fixed a bug where `podman build` was not properly picking up OCI runtime paths specified in `libpod.conf`
- Fixed a bug where Podman would mount `/dev/shm` into the container read-only for read-only containers (`/dev/shm` should always be read-write)
- Fixed a bug where Podman would ignore any mount whose container mountpoint was `/dev/shm`
- Fixed a bug where `podman export` did not work with the default `fuse-overlayfs` storage driver
- Fixed a bug where `podman inspect -f '{{ json .Config }}'` on images would not output anything (it now prints the image's config)
- Fixed a bug where `podman rmi -fa` displayed the wrong error message when trying to remove images used by pod infra containers

### Misc
- Rootless containers now unconditionally use postrun cleanup processes, ensuring resources are freed when the container stops
- A new version of Buildah is included for `podman build`, featuring improved build speed and numerous bugfixes
- Pulling images has been parallelized, allowing individual layers to be pulled in parallel
- The `podman start --attach` command now defaults the `sig-proxy` option to `true`, matching `podman create` and `podman run`
- The `podman info` command now prints the path of the configuration file controlling container storage
- Added `podman list` and `podman ls` as aliases for `podman ps`, and `podman container ps` and `podman container list` as aliases for `podman container ls`
- Changed `podman generate kube` to generate Kubernetes service YAML in the same file as pod YAML, generating a single file instead of two
- To improve compatibility with the Docker command line, `podman inspect -f '{{ json .ContainerConfig }}'` on images is no longer valid; please use `podman inspect -f '{{ json .Config }}'` instead

## 0.12.1.2
### Bugfixes
- Fixed a bug where an empty path for named volumes could make it impossible to create containers
- Fixed a bug where containers using another container's network namespace would not also use the other container's /etc/hosts and /etc/resolv.conf
- Fixed a bug where containers with `--rm` which failed to start were not removed
- Fixed a potential race condition attempting to read `/etc/passwd` inside containers

## 0.12.1.1
### Features
- Added the `podman generate kube` command to generate Kubernetes Pod and Service YAML for Podman containers and pods
- The `podman pod stop` flag now accepts a `--timeout` flag to set the timeout for stopping containers in the pod

### Bugfixes
- Fixed a bug where rootless Podman would fail to start if the default OCI hooks directory is not present

## 0.12.1
### Features
- Rootless Podman now creates the storage.conf, libpod.conf, and mounts.conf configuration files automatically in `~/.config/containers/` for ease of reconfiguration
- The `podman pod create` command can expose ports in the pod's network namespace, allowing public services to be created in pods
- The `podman container checkpoint` command can now keep containers running after they are checkpointed with the `--leave-running` flag
- The `podman container checkpoint` and `podman container restore` commands now support the `--tcp-established` flag to checkpoint and restore containers with active TCP connections
- The `podman version` command now has a `--format` flag to produce machine-readable output
- Added the `podman container exists`, `podman pod exists`, and `podman image exists` commands to easily check for a container/pod/image, respectively, by name or ID
- The `podman ps --pod` flag now has a short alias, `-p`
- The `podman rmi` and `podman rm` commands now have a `--prune` flag to prune unused images and containers, respectively
- The `podman ps` command now has a `--sync` flag to force a sync of Podman's state against the OCI runtime, resolving some state desync errors
- Added the `podman volume` set of commands for creating and managing local-only named volumes

### Bugfixes
- Fixed a breaking change in rootless Podman where a change in default paths caused Podman to be unable to function on systems upgraded from 0.10.x or earlier
- Fixed a bug where `podman exec` without `-t` would still use a terminal if the container was created with `-t`
- Fixed a bug where container root propagation was not being properly adjusted if volumes with root propagation set were mounted into the container
- Fixed a bug where `podman exec` could hold the container lock longer than necessary waiting for an exited container
- Fixed a bug where rootless containers using `slirp4netns` for networking were reporting using `bridge` networking in `podman inspect`
- Fixed a bug where `podman container restore -a` was attempting to restore all containers, including created and running ones. It will now only attempt to restore stopped and exited containers
- Fixed a bug where rootless Podman detached containers were not being properly cleaned up
- Fixed a bug where privileged containers were being mounted with incorrect (too restrictive) mount options such as `nodev`
- Fixed a bug where `podman stop` would throw an error attempting to stop a container that had already stopped
- Fixed a bug where `NOTIFY_SOCKET` was not properly being passed into Podman containers
- Fixed a bug where `/dev/shm` was not properly mounted in rootless containers
- Fixed a bug where rootless Podman would set up the CNI plugins for networking (despite not using them in rootless mode), potentially causing `inotify` related errors
- Fixed a bug where Podman would error on numeric GIDs that do not exist in the container's `/etc/group`
- Fixed a bug where containers in pods or created with `--net=container` were not mounting `/etc/resolv.conf` and `/etc/hosts`

### Misc
- `podman build` now defaults the `--force-rm` flag to `true`
- Improved `podman runlabel` support for labels featuring arguments with whitespace
- Containers without a network namespace will now use the host's `resolv.conf`
- The `slirp4netns` network mode can now be used with containers running as root. It may be useful for container-in-container scenarios where the outer container does not have host networking set
- Podman now uses `inotify` to wait for container exit files to be created, instead of polling. If `inotify` cannot be used, Podman will fall back to polling to check if the file has been created
- The `podman logs` command now uses improved short-options handling, allowing its flags to be combined if desired (for example, `podman logs -lf` instead of `podman logs -l -f`)
- Hardcoded OCI hooks directories used by Podman are now deprecated; they should instead be coded into the `libpod.conf` configuration file. They can be specified as an array via `hooks_dir`

## 0.11.1.1
### Bugfixes
- Fixed a bug where Podman was not correctly adding firewall rules for containers, preventing them from accessing the network
- Fixed a bug where full error messages were being lost when creating containers with user namespaces
- Fixed a bug where container state was not properly updated if a failure occurred during network setup, which could cause mounts to be left behind when the container was removed
- Fixed a bug where `podman exec` could time out on slower systems by increasing the relevant timeout

### Misc
- `podman rm -f` now removes paused containers. As such, `podman rm -af` completing successfully guarantees all Podman containers have been removed
- Added a field to `podman info` to show if Podman is being run as rootless
- Made a small output format change to `podman images` - image sizes now feature a space between number and unit (e.g. `123 MB` now instead of `123MB`)
- Vendored an updated version of `containers/storage` to fix several bugs reported upstream

## 0.11.1
### Features
- Added `--all` and `--latest` flags to `podman checkpoint` and `podman restore`
- Added `--max-workers` flag to all Podman commands that support operating in parallel, allowing the maximum number of parallel workers used to be specified
- Added `--all` flag to `podman restart`

### Bugfixes
- Fixed a bug where `podman port -l` would segfault if no containers were present
- Fixed a bug where `podman stats -a` would error if containers were present but not running
- Fixed a bug where container status checks would sometimes leave zombie OCI runtime processes
- Fixed checkpoint and restore code to verify an appropriate version of `criu` is being used
- Fixed a bug where environment variables with no specified value (e.g. `-e FOO`) caused errors (they are now added as empty)
- Fixed a bug where rootless Podman would attempt to configure the system firewall, causing errors on some systems where iptables is not in the user's PATH
- Fixed a bug where rootless Podman was unable to successfully write the container ID to a file when `--cid-file` was specified to `podman run`
- Fixed a bug where `podman unmount` would refuse to unmount a container if it was running (the unmount will now be deferred until the container stops)
- Fixed a bug where rootless `podman attach` would fail to attach due to a too-long path name
- Fixed a bug where `podman info` was not properly reporting the Git commit Podman was built from
- Fixed a bug where `podman run --interactive` was not holding STDIN open when `-a` flag was specified
- Fixed a bug where Podman with the `cgroupfs` CGroup driver was sometimes not successfully removing pod CGroups
- Fixed a bug where rootless Podman was unable to run systemd containers (note that this also requires an update to systemd)
- Fixed a bug where `podman run` with the `--user` flag would fail if the container image did not contain `/etc/passwd` or `/etc/group`

### Misc
- `podman rm`, `podman restart`, `podman kill`, `podman pause`, and `podman unpause` now operate in parallel, greatly improving speed when multiple containers are specified
- `podman create`, `podman run`, and `podman ps` have a number of improvements which should greatly increase their speed
- Greatly improved performance and reduced memory utilization of container status checks, which should improve the speed of most Podman commands
- Improve ability of `podman runlabel` to run commands that are not Podman
- Podman containers with an IP address now add their hostnames to `/etc/hosts`
- Changed default location of temporary libpod files in rootless Podman
- Updated the default Podman seccomp profile

### Compatibility
Several paths related to rootless Podman had their default values changed in this release.
If paths were not hardcoded in libpod.conf, your system may lose track of running containers and believe they are newly-created.

## 0.10.1.3
### Bugfixes
- Fixed a bug where `podman build` would not work while any containers were running

## 0.10.1.2
### Bugfixes
- Fixed cgroup mount for containers using systemd as init to work properly with the systemd cgroup manager

## 0.10.1.1
### Features
- Added handling for running containers as users with numeric UIDs not present in the container's /etc/passwd. This allows getpwuid() to work inside these containers.
- Added support for the REGISTRY_AUTH_FILE environment variable, which specifies the location of credentials for registry login. This is supported by the `push`, `pull`, `login`, `logout`, `runlabel`, and `search` commands

### Bugfixes
- Fixed handling for image volumes which are mounted on symlinks. The links are now resolved within the container, not on the host
- Fixed mounts for containers that use systemd as init to properly include all mounts required by systemd to function

### Misc
- Updated vendored version of Buildah used to power `podman build`

## 0.10.1
### Features
- Added the `podman container checkpoint` and `podman container restore` commands to checkpoint and restore containers
- Added the `podman container runlabel` command to run containers based on commands contained in their images
- Added the `podman create --ip` and `podman run --ip` flags to allow setting static IPs for containers
- Added the `podman kill --all` flag to send a signal to all running containers

### Bugfixes
- Fixed Podman cleanup processes for detached containers to properly print debug information when `--syslog` flag is specified
- Fixed manpages for `podman create` and `podman run` to document existing `--net` flag as an alias for `--network`
- Fixed issues with rootless Podman where specifying a single user mapping container was causing all Podman commands to hang
- Fixed an issue with rootless Podman not properly detecting when user namespaces were not enabled
- Fixed an issue where Podman user namespaces were not preserving file capabilities
- Fixed an issue where `resolv.conf` in container would unconditionally forward nameservers into the container, even localhost
- Fixed containers to release resources in the OCI runtime immediately after exiting, improving compatibility with Kata containers
- Fixed OCI runtime handling to fix several issues when using gVisor as an OCI runtime
- Fixed SELinux relabel errors when starting containers after a system restart
- Fixed a crash when initializing hooks on containers running systemd as init
- Fixed an SELinux labelling issue with privileged containers
- Fixed rootless Podman to raise better errors when using CGroup resource limits, which are not currently compatible with rootless
- Fixed a crash when runc was used as the OCI runtime for containers running systemd as init
- Fixed SELinux labelling for containers run with `--security-opt label=disable` to assign the correct label

### Misc
- Changed flag ordering on all Podman commands to ensure flags are alphabetized
- Changed `podman stop` to work in parallel when multiple containers are specified, greatly speeding up stop for containers that do not stop after SIGINT
- Updated vendored version of Buildah used to power `podman build`
- Added version of vendored Buildah to `podman info` to better debug issues

## 0.9.3.1
### Bugfixes
- Fixed a critical issue where SELinux contexts set on tmpfs volumes were causing runc crashes

## 0.9.3
### Features
- Added a flag to `libpod.conf`, `label`, to globally enable/disable SELinux labelling for libpod
- Added `--mount` flag to `podman create` and `podman run` as a new, more explicit way of specifying volume mounts

### Bugfixes
- Fixed a crash during container creation when an image had no names
- Fixed default rootfs mount propagation to for containers to match Docker
- Fixed permissions of `/proc` in containers
- Fixed permissions of some default bind mounts (for example, `/etc/hosts`) in read-only containers
- Fixed `/dev/shm` in `--ipc=container` and `--ipc=host` containers to use the correct SHM
- Fixed rootless Podman to properly join the namespaces of other containers
- Fixed the output of `podman diff` to not display some default changes that will not be committed
- Fixed rootless to better handle cases where insufficient UIDs/GIDs are mapped into the container

## 0.9.2.1
### Bugfixes
- Updated Buildah dependency to fix several bugs in `podman build`

### Misc
- Small performance improvement in image handling code to not recalculate digests

## 0.9.2
### Features
- Added `--interval` flag to `podman wait` to determine the interval between checks for container status
- Added a switch in `libpod.conf` to disable reserving ports for running containers. This lowers the safety of port allocations, but can significantly reduce memory usage.
- Added ability to search all the contents of a registry if no image name is specified when using `podman search`

### Bugfixes
- Further fixes for sharing of UTS namespaces within pods
- Fixed a deadlock in containers/storage that could be caused by numerous parallel Podman processes.
- Fixed Podman running into open file limits when many ports are forwarded
- Fixed default mount propagation on volume mounts
- Fixed default mounts under /dev remaining if /dev is bind-mounted into the container
- Fixed rootless `podman create` with no command specified throwing an error

### Misc
- Added `podman rm --volumes` flag for compatibility with Docker. As Podman does not presently support named volumes, this does nothing for now, but provides improved compatibility with the Docker command line.
- Improved error messages from `podman pull`

### Compatibility
- Podman is no longer being built by default with support for the Devicemapper storage driver. If you are using this storage driver, you should investigate switching to overlayfs.

## 0.9.1.1
### Bugfixes
- Added support for configuring iptables and firewalld firewalls to allow container traffic. This should resolve numerous issues with network access in containers.

### Note
It is recommended that you restart your system firewall after installing this release to clear any firewall rules created by older Podman versions. If port forwarding to containers does not work, it is recommended that you restart your system.

## 0.9.1
### Features
- Added initial support for the `podman pod` command as non-root

### Bugfixes
- Fixed regression where invalid Podman commands would still cause a clean exit
- Fixed `podman rmi --all` to not error if no images are present on the system
- Fixed parsing of container logs with `podman logs` to properly handle CRI logging, fixing some issues with blank lines in logs
- Fixed a bug creating pod cgroups using the systemd cgroup driver with systemd versions 239 and higher
- Fixed handling of volume mounts that overlapped with default container mounts (for example, `podman run -v /dev/:/dev`)
- Fixed sharing of UTS namespace in pods

### Misc
- Added additional debug information when pulling images if `--log-level=debug` is specified
- `podman build` now defaults to caching intermediate layers while building

## 0.8.5
### Features
- Added the ability to add a multipart entrypoint with `podman run --entrypoint`
- Improved help text when invalid commands are specified
- Greatly improved support for containers which use systemd as init

### Bugfixes
- Fixed several bugs with rootless `podman exec`
- Fixed rootless `podman` with a symlinked storage directory crashing
- Fixed bug with `podman ps` and multiple filters where the interface did not match Docker
- Fixed handling of `resolv.conf` on the host to handle symlinks
- Increased open file descriptor and process limits to match Docker and Buildah
- Fixed `podman run -h` to specify the container's hostname (as it does in Docker) instead of printing help text
- Fixed a bug with image shortname handling where repositories were incorrectly being treated as registries
- Fixed a bug where `podman wait` was busywaiting and consuming large amounts of CPU

## 0.8.4
### Features
- Added the `podman pod top` command
- Added the ability to easily share namespaces within a pod
- Added a pod statistics endpoint to the Varlink API
- Added information on container capabilities to the output of `podman inspect`

### Bugfixes
- Fixed a bug with the --device flag in `podman run` and `podman create`
- Fixed `podman pod stats` to accept partial pod IDs and pod names
- Fixed a bug with OCI hooks handling `ALWAYS` matches
- Fixed a bug with privileged rootless containers with `--net=host` set
- Fixed a bug where `podman exec --user` would not work with usernames, only numeric IDs
- Fixed a bug where Podman was forwarding both TCP and UDP ports to containers when protocol was not specified
- Fixed issues with Apparmor in rootless containers
- Fixed an issue with database encoding causing some containers created by Podman versions 0.8.1 and below to be unusable.

### Compatibility:
We switched JSON encoding/decoding to a new library for this release to address a compatibility issue introduced by v0.8.2.
However, this may cause issues with containers created in 0.8.2 and 0.8.3 with custom DNS servers.
