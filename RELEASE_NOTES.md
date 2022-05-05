# Release Notes

## 4.1.0
### Features
- Podman now supports Docker Compose v2.2 and higher ([#11822](https://github.com/containers/podman/issues/11822)). Please note that it may be necessary to disable the use of Buildkit by setting the environment variable `DOCKER_BUILDKIT=0`.
- A new container command has been added, `podman container clone`. This command makes a copy of an existing container, with the ability to change some settings (e.g. resource limits) while doing so.
- A new machine command has been added, `podman machine inspect`. This command provides details on the configuration of machine VMs.
- The `podman machine set` command can now change the CPUs, memory, and disk space available to machines after they were initially created, using the new `--cpus`, `--disk-size`, and `--memory` options ([#13633](https://github.com/containers/podman/issues/13633)).
- Podman now supports sending JSON events related to machines to a Unix socket named `machine_events.*\.sock` in `XDG_RUNTIME_DIR/podman` or to a socket whose path is set in the `PODMAN_MACHINE_EVENTS_SOCK` environment variable.
- Two new volume commands have been added, `podman volume mount` and `podman volume unmount`. These allow for Podman-managed named volumes to be mounted and accessed from outside containers ([#12768](https://github.com/containers/podman/issues/12768)).
- VMs created by `podman machine` now automatically mount the host's `$HOME` into the VM, to allow mounting volumes from the host into containers.
- The `podman container checkpoint` and `podman container restore` options now support checkpointing to and restoring from OCI images. This allows checkpoints to be distributed via standard image registries.
- The `podman play kube` command now supports environment variables that are specified using the `fieldRef` and `resourceFieldRef` sources.
- The `podman play kube` command will now set default resource limits when the provided YAML does not include them ([#13115](https://github.com/containers/podman/issues/13115)).
- The `podman play kube` command now supports a new option, `--annotation`, to add annotations to created containers ([#12968](https://github.com/containers/podman/issues/12968)).
- The `podman play kube --build` command now supports a new option, `--context-dir`, which allows the user to specify the context directory to use when building the Containerfile ([#12485](https://github.com/containers/podman/issues/12485)).
- The `podman container commit` command now supports a new option, `--squash`, which squashes the generated image into a single layer ([#12889](https://github.com/containers/podman/issues/12889)).
- The `podman pod logs` command now supports two new options, `--names`, which identifies which container generated a log message by name, instead of ID ([#13261](https://github.com/containers/podman/issues/13261)) and `--color`, which colors messages based on what container generated them ([#13266](https://github.com/containers/podman/issues/13266)).
- The `podman rmi` command now supports a new option, `--ignore`, which will ignore errors caused by missing images.
- The `podman network create` command now features a new option, `--ipam-driver`, to specify details about how IP addresses are assigned to containers in the network ([#13521](https://github.com/containers/podman/issues/13521)).
- The `podman machine list` command now features a new option, `--quiet`, to print only the names of configured VMs and no other information.
- The `--ipc` option to the `podman create`, `podman run`, and `podman pod create` commands now supports three new modes: `none`, `private`, and `shareable`. The default IPC mode is now `shareable`, indicating the the IPC namespace can be shared with other containers ([#13265](https://github.com/containers/podman/issues/13265)).
- The `--mount` option to the `podman create` and `podman run` commands can now set options for created named volumes via the `volume-opt` parameter ([#13387](https://github.com/containers/podman/issues/13387)).
- The `--mount` option to the `podman create` and `podman run` commands now allows parameters to be passed in CSV format ([#13922](https://github.com/containers/podman/issues/13922)).
- The `--userns` option to the `podman create` and `podman run` commands now supports a new option, `nomap`, that (only for rootless containers) does not map the UID of the user that started the container into the container, increasing security.
- The `podman import` command now supports three new options, `--arch`, `--os`, and `--variant`, to specify what system the imported image was built for.
- The `podman inspect` command now includes information on the network configuration of containers that joined a pre-configured network namespace with the `--net ns:` option to `podman run`, `podman create`, and `podman pod create`.
- The `podman run` and `podman create` commands now support a new option, `--chrootdirs`, which specifies additional locations where container-specific files managed by Podman (e.g. `/etc/hosts`, `/etc/resolv.conf, etc) will be mounted inside the container ([#12961](https://github.com/containers/podman/issues/12691)).
- The `podman run` and `podman create` commands now support a new option, `--passwd-entry`, allowing entries to be added to the container's `/etc/passwd` file.
- The `podman images --format` command now accepts two new format directives: `{{.CreatedAt}}` and `{{.CreatedSince}}` ([#14012](https://github.com/containers/podman/issues/14012)).
- The `podman volume create` command's `-o` option now accepts a new argument, `o=noquota`, to disable XFS quotas entirely and avoid potential issues when Podman is run on an XFS filesystem with existing quotas defined ([#14049](https://github.com/containers/podman/issues/14049)).
- The `podman info` command now includes additional information on the machine Podman is running on, including disk utilization on the drive Podman is storing containers and images on, and CPU utilization ([#13876](https://github.com/containers/podman/issues/13876)).

### Changes
- The `--net=container:` option to `podman run`, `podman create`, and `podman pod create` now conflicts with the `--add-host` option.
- As part of a deprecation of the SHA1 hash algorithm within Podman, the algorithm used to generate the filename of the rootless network namespace has been changed. As a result, rootless containers started before updating to Podman 4.1.0 will need to be restarted if they are joined to a network (and not just using `slirp4netns`) to ensure they can connect to containers started the upgrade.
- Podman's handling of the `/etc/hosts` file has been rewritten to improve its consistency and handling of edge cases ([#12003](https://github.com/containers/podman/issues/12003) and [#13224](https://github.com/containers/podman/issues/13224)). As part of this, two new options are available in `containers.conf`: `base_hosts_file` (to specify a nonstandard location to source the base contents of the container's `/etc/hosts`) and `host_containers_internal_ip` (to specify a specific IP address for containers' `host.containers.internal` entry to point to).
- The output of the `podman image trust show` command now includes information on the transport mechanisms allowed.
- Podman now exits cleanly (with exit code 0) after receiving SIGTERM.
- Containers running in systemd mode now set the `container_uuid` environment variable ([#13187](https://github.com/containers/podman/issues/13187)).
- Renaming a container now generates an event readable through `podman events`.
- The `--privileged` and `--cap-add` flags are no longer mutually exclusive ([#13449](https://github.com/containers/podman/issues/13449)).
- Fixed a bug where the `--mount` option to `podman create` and `podman run` could not create anonymous volumes ([#13756](https://github.com/containers/podman/issues/13756)).
- Fixed a bug where Podman containers where the user did not explicitly set an OOM score adjustment would implicitly set a value of 0, instead of not setting one at all ([#13731](https://github.com/containers/podman/issues/13731)).
- The `podman machine set` command can no longer be used while the VM being updated is running ([#13783](https://github.com/containers/podman/issues/13783)).
- Systemd service files created by `podman generate systemd` are now prettyprinted for increased readability.
- The `file` event log driver now automatically rotates the log file, preventing it from growing beyond a set size.
- The `--no-trunc` flag to `podman search` now defaults to `false`, to ensure output is not overly verbose.

### Bugfixes
- Fixed a bug where Podman could not add devices with a major or minor number over 256 to containers.
- Fixed a bug where containers created by the `podman play kube` command did not record the raw image name used to create containers.
- Fixed a bug where VMs created by `podman machine` could not start containers which forwarded ports when run on a host with a proxy configured ([#13628](https://github.com/containers/podman/issues/13628)).
- Fixed a bug where VMs created by the `podman machine` command could not be connected to when the username of the current user was sufficiently long ([#12751](https://github.com/containers/podman/issues/12751)).
- Fixed a bug where the `podman system reset` command on Linux did not fully remove virtual machines created by `podman machine`.
- Fixed a bug where the `podman machine rm` command would error when removing a VM that was never started ([#13834](https://github.com/containers/podman/issues/13834)).
- Fixed a bug where the remote Podman client's `podman manifest push` command could not push to registries that required authentication ([#13629](https://github.com/containers/podman/issues/13629)).
- Fixed a bug where containers joining a pod with volumes did not have the pod's volumes added ([#13548](https://github.com/containers/podman/issues/13548)).
- Fixed a bug where the `podman version --format` command could not return the OS of the server ([#13690](https://github.com/containers/podman/issues/13690)).
- Fixed a bug where the `podman play kube` command would error when a volume specified by a `configMap` already existed ([#13715](https://github.com/containers/podman/issues/13715)).
- Fixed a bug where the `podman play kube` command did not respect the `hostNetwork` setting in Pod YAML ([#14015](https://github.com/containers/podman/issues/14015)).
- Fixed a bug where the `podman play kube` command would, when the `--log-driver` flag was not specified, ignore Podman's default log driver ([#13781](https://github.com/containers/podman/issues/13781)).
- Fixed a bug where the `podman generate kube` command could generate YAML with too-long labels ([#13962](https://github.com/containers/podman/issues/13962)).
- Fixed a bug where the `podman logs --tail=1` command would fail when the log driver was `journald` and the container was restarted ([#13098](https://github.com/containers/podman/issues/13098)).
- Fixed a bug where containers created from images with a healthcheck that did not specify an interval would never run their healthchecks ([#13912](https://github.com/containers/podman/issues/13912)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands could leave invalid entries in `/etc/hosts` ([#13533](https://github.com/containers/podman/issues/13533)).
- Fixed a bug where the `--tls-verify option to the `remote Podman client's `podman build` command was nonfunctional.
- Fixed a bug where the `podman pod inspect` command incorrectly reported whether the pod used the host's network ([#14028](https://github.com/containers/podman/issues/14028)).
- Fixed a bug where Podman would, when run on WSL2, ports specified without an IP address (e.g. `-p 8080:8080`) would be bound to IPv6 addresses ([#12292](https://github.com/containers/podman/issues/12292)).
- Fixed a bug where the remote Podman client's  `podman info` could report an incorrect path to the socket used to access the Podman service ([#12023](https://github.com/containers/podman/issues/12023)).

### API
- Containers created via the Libpod Create API that set a memory limit, but not a swap limit, will automatically have a swap limit set ([#13145](https://github.com/containers/podman/issues/13145)).
- The Compat and Libpod Attach APIs for Containers can now attach to Stopped containers.
- Fixed a bug where the Compat and Libpod Create APIs for Containers did not respect the `no_hosts` option in `containers.conf` ([#13719](https://github.com/containers/podman/issues/13719)).
- Fixed a bug where the default network mode for rootless containers created via the Compat Create API was not `bridge`.
- Fixed a bug where the Libpod List API for Containers did not allow filtering based on the `removing` status ([#13986](https://github.com/containers/podman/issues/13986)).
- Fixed a bug where the Libpod Modify endpoint for Manifests did not respect the `tlsVerify` parameter.

### Misc
- A number of dependencies have been pruned from the project, resulting in a significant reduction in the size of the Podman binary.
- Using `podman play kube` on a YAML that only includes `configMap` objects (and no pods or deployments) now prints a much clearer error message.
- Updated Buildah to v1.26.1
- Updated the containers/storage library to v1.40.2
- Updated the containers/image library to v5.21.1
- Updated the containers/common library to v0.48.0

## 4.0.3
### Security
- This release fixes CVE-2022-27649, where containers run by Podman would have excess inheritable capabilities set.

### Changes
- The `podman machine rm --force` command will now remove running machines as well (such machines are shut down first, then removed) ([#13448](https://github.com/containers/podman/issues/13448)).
- When a `podman machine` VM is started that is using a too-old VM image, it will now start in a reduced functionality mode, and provide instructions on how to recreate it (previously, VMs were effectively unusable) ([#13510](https://github.com/containers/podman/issues/13510)).

### Bugfixes
- Fixed a bug where devices added to containers by the `--device` option to `podman run` and `podman create` would not be accessible within the container.
- Fixed a bug where Podman would refuse to create containers when the working directory in the container was a symlink ([#13346](https://github.com/containers/podman/issues/13346)).
- Fixed a bug where pods would be created with cgroups even if cgroups were disabled in `containers.conf` ([#13411](https://github.com/containers/podman/issues/13411)).
- Fixed a bug where the `podman play kube` command would produce confusing errors if invalid YAML with duplicated container named was passed ([#13332](https://github.com/containers/podman/issues/13332)).
- Fixed a bug where the `podman machine rm` command would not remove the Podman API socket on the host that was associated with the VM.
- Fixed a bug where the remote Podman client was unable to properly resize the TTYs of containers on non-Linux OSes.
- Fixed a bug where rootless Podman could hang indefinitely when starting containers on systems with IPv6 disabled ([#13388](https://github.com/containers/podman/issues/13388)).
- Fixed a bug where the `podman version` command could sometimes print excess blank lines as part of its output.
- Fixed a bug where the `podman generate systemd` command would sometimes generate systemd services with names beginning with a hyphen ([#13272](https://github.com/containers/podman/issues/13272)).
- Fixed a bug where locally building the pause image could fail if the current directory contained a `.dockerignore` file ([#13529](https://github.com/containers/podman/issues/13529)).
- Fixed a bug where root containers in VMs created by `podman machine` could not bind ports to specific IPs on the host ([#13543](https://github.com/containers/podman/issues/13543)).
- Fixed a bug where the storage utilization percentages displayed by `podman system df` were incorrect ([#13516](https://github.com/containers/podman/issues/13516)).
- Fixed a bug where the CPU utilization percentages displayed by `podman stats` were incorrect ([#13597](https://github.com/containers/podman/pull/13597)).
- Fixed a bug where containers created with the `--no-healthcheck` option would still display healthcheck status in `podman inspect` ([#13578](https://github.com/containers/podman/issues/13578)).
- Fixed a bug where the `podman pod rm` command could print a warning about a missing cgroup ([#13382](https://github.com/containers/podman/issues/13382)).
- Fixed a bug where the `podman exec` command could sometimes print a `timed out waiting for file` error after the process in the container exited ([#13227](https://github.com/containers/podman/issues/13227)).
- Fixed a bug where virtual machines created by `podman machine` were not tolerant of changes to the path to the qemu binary on the host ([#13394](https://github.com/containers/podman/issues/13394)).
- Fixed a bug where the remote Podman client's `podman build` command did not properly handle the context directory if a Containerfile was manually specified using `-f` ([#13293](https://github.com/containers/podman/issues/13293)).
- Fixed a bug where Podman would not properly detect the use of `systemd` as PID 1 in a container when the entrypoint was prefixed with `/bin/sh -c` ([#13324](https://github.com/containers/podman/issues/13324)).
- Fixed a bug where rootless Podman could, on systems that do not use `systemd` as init, print a warning message about the rootless network namespace ([#13703](https://github.com/containers/podman/issues/13703)).
- Fixed a bug where the default systemd unit file for `podman system service` did not delegate all cgroup controllers, resulting in `podman info` queries against the remote API returning incorrect cgroup controllers ([#13710](https://github.com/containers/podman/issues/13710)).
- Fixed a bug where the `slirp4netns` port forwarder for rootless Podman would only publish the first port of a range ([#13643](https://github.com/containers/podman/issues/13643)).

### API
- Fixed a bug where the Compat Create API for containers did not properly handle permissions for tmpfs mounts ([#13108](https://github.com/containers/podman/issues/13108)).

### Misc
- The static binary for Linux is now built with CGo disabled to avoid panics due to a Golang bug ([#13557](https://github.com/containers/podman/issues/13557)).
- Updated Buildah to v1.24.3
- Updated the containers/storage library to v1.38.3
- Updated the containers/image library to v5.19.2
- Updated the containers/common library to v0.47.5

## 4.0.2
### Bugfixes
- Revert "use GetRuntimeDir() from c/common"

## 4.0.1
### Bugfixes
- Fixed a bug where the `podman play kube` command did not honor the `mountPropagation` field in Pod YAML ([#13322](https://github.com/containers/podman/issues/13322)).
- Fixed a bug where the `--build=false` option to `podman play kube` was not honored ([#13285](https://github.com/containers/podman/issues/13285)).
- Fixed a bug where a container using volumes from another container (via `--volumes-from`) could, under certain circumstances, exit with errors that it could not delete some volumes if the other container did not exit before it ([#12808](https://github.com/containers/podman/issues/12808)).
- Fixed a bug where the `CONTAINERS_CONF` environment variable was not propagated to Conmon, which could result in Podman cleanup processes being run with incorrect configurations.

## 4.0.0
### Security
- This release addresses CVE-2022-1227, where running `podman top` on a container made from a maliciously-crafted image and using a user namespace could allow for code execution in the host context.

### Features
- Podman has seen an extensive rewrite of its network stack to add support for Netavark, a new tool for configuring container networks, in addition to the existing CNI stack. Netavark will be default on new installations when it is available.
- The `podman network connect` command now supports three new options, `--ip`, `--ip6`, and `--mac-address`, to specify configuration for the new network that will be attached.
- The `podman network create` command now allows the `--subnet`, `--gateway`, and `--ip-range` options to be specified multiple times, to allow for the creation of dual-stack IPv4 and IPv6 networks with user-specified subnets.
- The `--network` option to `podman create`, `podman pod create`, `podman run`, and `podman play kube` can now, when specifying a network name, also specify advanced network options such as `alias`, `ip`, `mac`, and `interface_name`, allowing advanced configuration of networks when creating containers connected to more than one network.
- The `podman play kube` command can now specify the `--net` option multiple times, to connect created containers and pods to multiple networks.
- The `podman create`, `podman pod create`, and `podman run` commands now support a new option, `--ip6`, to specify a static IPv6 address for the created container or pod to use.
- Macvlan networks can now configure the mode of the network via the `-o mode=` option.
- When using the CNI network stack, a new network driver, `ipvlan`, is now available.
- The `podman info` command will now print the network backend in use (Netavark or CNI).
- The network backend to use can be now be specified in `containers.conf` via the `network_backend` field. Please note that it is not recommended to switch backends while containers exist, and a system reboot is recommended after doing so.
- All Podman commands now support a new option, `--noout`, that suppresses all output to STDOUT.
- All commands that can remove containers (`podman rm --force`, `podman pod rm --force`, `podman volume rm --force`, `podman network rm --force`) now accept a `--time` option to specify the timeout on stopping the container before resorting to `SIGKILL` (identical to the `--time` flag to `podman stop`).
- The `podman run` and `podman create` commands now support a new option, `--passwd`, that uses the `/etc/passwd` and `/etc/groups` files from the image in the created container without changes by Podman ([#11805](https://github.com/containers/podman/issues/11805)).
- The `podman run` and `podman create` commands now support a new option, `--hostuser`, that creates one or more users in the container based on users from the host (e.g. with matching username, UID, and GID).
- The `podman create` and `podman run` commands now support two new options, `--unsetenv` and `--unsetenv-all`, to clear default environment variables set by Podman and by the container image ([#11836](https://github.com/containers/podman/issues/11836)).
- The `podman rm` command now supports a new option, `--depend`, which recursively removes a given container and all containers that depend on it ([#10360](https://github.com/containers/podman/issues/10360)).
- All commands that support filtering their output based on labels (e.g. `podman volume ls`, `podman ps`) now support labels specified using regular expressions (e.g. `--filter label=some.prefix.com/key/*`).
- The `podman pod create` command now supports the `--volume` option, allowing volumes to be specified that will be mounted automatically to all containers in the pod ([#10379](https://github.com/containers/podman/issues/10379)).
- The `podman pod create` command now supports the `--device` option, allowing devices to be specified that will be mounted automatically to all containers in the pod.
- The `podman pod create` command now supports the `--volumes-from` option, allowing volumes from an existing Podman container to be mounted automatically to all containers in the pod.
- The `podman pod create` command now supports the `--security-opt` option, allowing security settings (e.g. disabling SELinux or Seccomp) to be configured automatically for all containers in the pod ([#12173](https://github.com/containers/podman/issues/12173)).
- The `podman pod create` command now supports the `--share-parent` option, which defaults to true, controlling whether containers in the pod will use a shared cgroup parent.
- The `podman pod create` command now supports the `--sysctl` option, allowing sysctls to be configured automatically for all containers in the pod.
- The `podman events` command now supports the `--no-trunc` option, which will allow short container IDs to be displayed instead of the default full IDs. The flag defaults to true, so full IDs remain the default ([#8941](https://github.com/containers/podman/issues/8941)).
- The `podman machine init` command now supports a new VM type, `wsl`, available only on Windows; this uses WSL as a backend for `podman machine`, instead of creating a separate VM and managing it via QEMU ([#12503](https://github.com/containers/podman/pull/12503)).
- The `podman machine init` command now supports a new option, `--now`, to start the VM immediately after creating it.
- The `podman machine init` command now supports a new option, `--volume`, to mount contents from the host into the created virtual machine.
- Virtual machines created by `podman machine` now automatically mount the Podman API socket to the host, so consumers of the Podman or Docker APIs can use them directly from the host machine ([#11462](https://github.com/containers/podman/issues/11462)).
- Virtual machines created by `podman machine` now automatically mount certificates from the host's keychain into the virtual machine ([#11507](https://github.com/containers/podman/issues/11507)).
- Virtual machines created by `podman machine` now automatically propagate standard proxy environment variables from the host into the virtual machine, including copying any required certificates from `SSL_FILE_CERT` into the VM.
- The `podman machine ssh` command now supports a new option, `--username`, to specify the username to connect to the VM with.
- Port forwarding from VMs created using `podman machine` now supports ports specified using custom host IPs (e.g. `-p 127.0.0.1:8080:80`), the UDP protocol, and containers created using the `slirp4netns` network mode ([#11528](https://github.com/containers/podman/issues/11528) and [#11728](https://github.com/containers/podman/issues/11728)).
- The `podman system connection rm` command supports a new option, `--all`, to remove all available connections ([#12018](https://github.com/containers/podman/issues/12018)).
- The `podman system service` command's default timeout is now configured via `containers.conf` (using the `service_timeout` field) instead of hardcoded to 5 seconds.
- The `--mount type=devpts` option to `podman create` and `podman run` now supports new options: `uid`, `gid`, `mode`, and `max`.
- The `--volume` option to `podman create` and `podman run` now supports a new option, `:idmap`, which using an ID mapping filesystem to allow multiple containers with disjoint UID and GID ranges mapped into them access the same volume ([#12154](https://github.com/containers/podman/issues/12154)).
- The `U` option for volumes, which changes the ownership of the mounted volume to ensure the user running in the container can access it, can now be used with the `--mount` option to `podman create` and `podman run`, as well as the `--volume` option where it was already available.
- The `:O` option for volumes, which specifies that an overlay filesystem will be mounted over the volume and ensures changes do not persist, is now supported with named volumes as well as bind mounts.
- The `:O` option for volumes now supports two additional options, `upperdir` and `workdir`, which allow for specifying custom upper directories and work directories for the created overlay filesystem.
- Podman containers created from a user-specified root filesystem (via `--rootfs`) can now create an overlay filesystem atop the user-specified rootfs which ensures changes will not persist by suffixing the user-specified root filesystem with `:O`.
- The `podman save` command has a new option, `--uncompressed`, which saves the layers of the image without compression ([#11613](https://github.com/containers/podman/issues/11613)).
- Podman supports a new log driver for containers, `passthrough`, which logs all output directly to the STDOUT and STDERR of the `podman` command; it is intended for use in systemd-managed containers.
- The `podman build` command now supports two new options, `--unsetenv` and `--all-platforms`.
- The `podman image prune` command now supports a new option, `--external`, which allows containers not created by Podman (e.g. temporary containers from Buildah builds) to be pruned ([#11472](https://github.com/containers/podman/issues/11472)).
- Two new aliases for `podman image prune` have been added for Docker compatibility: `podman builder prune` and `podman buildx prune`.
- The `podman play kube` command now supports a new option, `--no-hosts`, which uses the `/etc/hosts` file from the image in all generated containers, preventing any modifications to the hosts file from Podman ([#9500](https://github.com/containers/podman/issues/9500)).
- The `podman play kube` command now supports a new option, `--replace`, which will replace any existing containers and pods with the same names as the containers and pods that will be created by the command ([#11481](https://github.com/containers/podman/issues/11481)).
- The `podman play kube` command now supports a new option, `--log-opt`, which allows the logging configuration of generated containers and pods to be adjusted ([#11727](https://github.com/containers/podman/issues/11727)).
- The `podman play kube` command now supports Kubernetes YAML that specifies volumes from a configmap.
- The `podman generate systemd` command now supports a new option, `--template`, to generate template unit files.
- The `podman generate systemd` command now supports a new option, `--start-timeout`, to override the default start timeout for generated unit files ([#11618](https://github.com/containers/podman/issues/11618)).
- The `podman generate systemd` command now supports a new option, `--restart-sec`, to override the default time before a failed unit is restarted by systemd for generated unit files.
- The `podman generate systemd` command now supports three new options, `--wants`, `--after`, and `--requires`, which allow detailed control of systemd dependencies in generated unit files.
- The `podman container checkpoint` and `podman container restore` commands can now print statistics about the checkpoint operation via a new option, `--print-stats`.
- The `podman container checkpoint` and `podman container restore` commands can now checkpoint and restore containers which make use of file locks via a new option, `--file-locks`.
- The `podman container restore` command can now be used with containers created using the host IPC namespace (`--ipc=host`).
- The `podman container checkpoint` and `podman container restore` commands now handle checkpointing and restoring the contents of `/dev/shm`.
- The `podman container checkpoint` and `podman container restore` commands are now supported with the remote Podman client ([#12007](https://github.com/containers/podman/issues/12007)).
- The `podman inspect` command on containers now includes additional output fields for checkpointed and restored containers, including information about when the container was checkpointed or restored, and the path to the checkpoint/restore log.
- The `podman secret list` command now supports a new option, `--filter`, to filter what secrets are returned.
- The `podman image scp` command can now be used to transfer images between users (both root and rootless) on the same system, without requiring `sshd`.
- The `podman image sign` command now supports a new option, `--authfile`, to specify an alternative path to authentication credentials ([#10866](https://github.com/containers/podman/issues/10866)).
- The `podman load` command now supports downloading files via HTTP and HTTPS if a URL is given ([#11970](https://github.com/containers/podman/issues/11970)).
- The `podman push` command now supports a new option, `--compression-format`, to choose the compression algorithm used to compress image layers.
- The `podman volume create` command now allows volumes using the `local` driver that require mounting to be used by non-root users. This allows `tmpfs` and `bind` volumes to be created by non-root users ([#12013](https://github.com/containers/podman/issues/12013)).
- A new command, `podman dial-stdio`, has been added; this command should not be invoked directly, but is used by some clients of the Docker Remote API, and is provided for Docker compatibility ([#11668](https://github.com/containers/podman/issues/11668)).

### Breaking Changes
- Podman v4.0 will perform several schema migrations in the Podman database when it is first run. These schema migrations will cause Podman v3.x and earlier to be unable to read certain network configuration information from the database, so downgrading from Podman v4.0 to an earlier version will cause containers to lose their static IP, MAC address, and port bindings.
- All endpoints of the Docker-compatible API now enforce that all image shortnames will be resolved to the Docker Hub for improved Docker compatibility. This behavior can be turned off via the `compat_api_enforce_docker_hub` option in `containers.conf` ([#12320](https://github.com/containers/podman/issues/12320)).
- The Podman APIs for Manifest List and Network operations have been completely rewritten to address issues and inconsistencies in the previous APIs. Incompatible APIs should warn if they are used with an older Podman client.
- The `make install` makefile target no longer implicitly builds Podman, and will fail if `make` was not run prior to it.
- The `podman rm --depends`, `podman rmi --force`, and `podman network rm --force` commands can now remove pods if a they need to remove an infra container (e.g. `podman rmi --force` on the infra image will remove all pods and infra containers). Previously, any command that tried to remove an infra container would error.
- The `podman system reset` command now removes all networks on the system, in addition to all volumes, pods, containers, and images.
- If the `CONTAINER_HOST` environment variable is set, Podman will default to connecting to the remote Podman service specified by the environment variable, instead of running containers locally ([#11196](https://github.com/containers/podman/issues/11196)).
- Healthcheck information from `podman inspect` on a container has had its JSON tag renamed from `Healthcheck` to `Health` for improved Docker compatibility. An alias has been added so that using the old name with the `--format` option will still work ([#11645](https://github.com/containers/podman/issues/11645)).
- Secondary IP and IPv6 addresses from `podman inspect` on a container (`SecondaryIPAddresses` and `SecondaryIPv6Addresses`) have been changed from arrays of strings to arrays of structs for improved Docker compatibility (the struct now includes IP address and prefix length).
- The `podman volume rm --force` command will now remove containers that depend on the volume that are running (previously, it would only remove stopped containers).
- The output of the `podman search` command has been altered to remove the Index, Stars, and Automated columns, as these were not used by registries that are not Dockerhub.
- The `host.containers.internal` entry in `/etc/hosts` for rootless containers now points to a public IP address of the host machine, to ensure the container can reach the host (the previous value, a slirp4netns address, did not actually point to the host) ([#12000](https://github.com/containers/podman/issues/12000)).
- Containers created in pods that have an infra container can no longer independently configure a user namespace via `--uidmap` and `--gidmap` ([#12669](https://github.com/containers/podman/issues/12669)).
- Several container states have been renamed internally - for example, the previous `Configured` state is now named `Created`, and the previous `Created` state is now `Initialized`. The `podman ps` command already normalized these names for Docker compatibility, so this will only be visible when inspecting containers with `podman inspect`.

### Changes
- Podman containers will now automatically add the container's short ID as a network alias when connected to a supporting network ([#11748](https://github.com/containers/podman/issues/11748)).
- The `podman machine stop` command will now log when machines are successfully stopped ([#11542](https://github.com/containers/podman/issues/11542)).
- The `podman machine stop` command now waits until the VM has stopped to return; previously, it returned immediately after the shutdown command was sent, without waiting for the VM to shut down.
- VMs created by `podman machine` now delegate more cgroup controllers to the rootless user used to run containers, allowing for additional resource limits to be used ([#13054](https://github.com/containers/podman/issues/13054)).
- The `podman stop` command will now log a warning to the console if the stop timeout expires and `SIGKILL` must be used to stop the container ([#11854](https://github.com/containers/podman/issues/11854)).
- Several performance optimizations have been implemented that should speed up container and pod creation, and running containers and pods that forward large ranges of ports.
- The `--no-trunc` argument to the `podman search` command now defaults to true.
- Rootless port forwarding using the `rootlessport` port forwarder is now handled by a separate binary, not Podman itself, which results in significantly reduced memory usage ([#10790](https://github.com/containers/podman/issues/10790)).
- The `podman system connection ls` command now has a separate output column to show which connection is currently the default (instead appending `*` to the default connection's name) ([#12019](https://github.com/containers/podman/issues/12019)).
- The `--kernel-memory` option to `podman run` and `podman create` has been deprecated in the upstream OCI runtime specification, and is now also deprecated in Podman and will be removed in a future release. Use of the flag will result in a warning.
- Podman will now ship build the pause image used by pods locally, instead of pulling it from the network (using the existing `catatoinit` binary used for `podman run --init`). This allows pods to be easily used on systems without an internet connection.
- The `--rootless-cni` option to `podman unshare` has been renamed to `--rootless-netns`. The old name has been aliased to the new one and will still function, but may be removed in a future release.
- The `--cni-config-dir` option to all Podman commands has been renamed to `--network-config-dir` as it will not be used with Netavark as well as CNI. The old name has been aliased to the new one and will still function, but may be removed in a future release.
- The `--format` option to all Podman commands has been changed to improved functionality and Docker compatibility ([#10974](https://github.com/containers/podman/issues/10974)).
- The `podman ps --external` flag previously required `--all` to also be specified; this is no longer true
- The port-forwarding logic previously contined in the `podman-machine-cni` CNI plugin has been integrated directly into Podman. The `podman-machine-cni` plugin is no longer necessary and should be removed.
- The `--device` flag to `podman create`, `podman run`, and `podman pod create` would previously refuse to mount devices when Podman was run as a non-root user and no permission to access the device was available; it will now mount these devices without checking permissions ([#12704](https://github.com/containers/podman/issues/12704)).

### Bugfixes
- Fixed a bug where networks could be created with the same name as a container network mode (e.g. `host`) ([#11448](https://github.com/containers/podman/issues/11448)).
- Fixed a bug where the `podman save` command was not automatically removing signatures from saved images.
- Fixed a bug where a rare race condition could cause `podman run --rm` to return an error that a given container did not exist when trying to remove it, despite it having been safely removed ([#11775](https://github.com/containers/podman/issues/11775)).
- Fixed a bug where a rare race condition could cause `podman ps` to return an error if a container was removed while the command was running ([#11810](https://github.com/containers/podman/issues/11810)).
- Fixed a bug where running Kube YAML with a CPU limit would using `podman play kube` would result in errors ([#11803](https://github.com/containers/podman/issues/11803)).
- Fixed a bug where creating a pod without an infra container would not generate an Pod Create event.
- Fixed a bug where volumes created with the `:z` and `:Z` options would be relabelled every time a container was started, not just the first time.
- Fixed a bug where the `podman tag` command on a manifest list could tag an image in the manifest, and not the manifest list itself.
- Fixed a bug where creating a volume using an invalid volume option that contained a format string would print a nonsensical error.
- Fixed a bug where Podman would not create a healthcheck for containers created from images that specified a healthcheck in their configuration ([#12226](https://github.com/containers/podman/issues/12226)).
- Fixed a bug where the output of healthchecks was not shown in `podman inspect` ([#13083](https://github.com/containers/podman/issues/13083)).
- Fixed a bug where rootless containers that used a custom user namespace (e.g. `--userns=keep-id`) could not have any ports forwarded to them.
- Fixed a bug where the `podman system connection ls` command would not print any output (including headers) if no connections were present.
- Fixed a bug where the `--memory-swappiness` option to `podman create` and `podman run` did not accept 0 as a valid value.
- Fixed a bug where environment variables specified in `containers.conf` for Podman would sometimes not be applied ([#12296](https://github.com/containers/podman/issues/12296)).
- Fixed a bug where running multiple rootless Podman instances with different configurations on the same system could cause networking issues due to the use of a single, shared rootless network namespace ([#12306](https://github.com/containers/podman/issues/12306)).
- Fixed a bug where rootless containers using bridge networking would fail if `/etc/resolv.conf` was a symlink to a directory ([#12461](https://github.com/containers/podman/issues/12461)).
- Fixed a bug where `podman container restore` could sometimes restore containers with a different OCI runtime than they had been using before they were checkpointed.
- Fixed a bug where some commands of the remote Podman client allowed the `--signature-policy` option to be used (with no effect); `--signature-policy` is not supported by the remote client ([#12357](https://github.com/containers/podman/issues/12357)).
- Fixed a bug where images which specified a port range in `EXPOSE` could not be run ([#12293](https://github.com/containers/podman/issues/12293)).
- Fixed a bug where Podman would resolve image names without a tag to any tag of that image available on the local system, instead of the `:latest` tag ([#11964](https://github.com/containers/podman/issues/11964)).
- Fixed a bug where the `--blkio-weight-device` option to `podman create` and `podman run` was nonfunctional.
- Fixed a bug where the `podman generate systemd` command did not support container entrypoints that were specified as JSON arrays ([#12477](https://github.com/containers/podman/issues/12477)).
- Fixed a bug where rootless Podman could, under some circumstances, exhaust all available inotify watches ([#11825](https://github.com/containers/podman/issues/11825)).
- Fixed a bug where, when a container was created with both the `--hostname` and `--pod new:` options, the hostname would be discarded; it is now set as the hostname of the created pod, which will be used by the container.
- Fixed a bug where the order in which `podman network ls` printed networks was not deterministic.
- Fixed a bug where the `podman kill` command would sometimes not print the ID of containers that were killed.
- Fixed a bug where VMs created by `podman machine` did not match their timezone to the host system ([#11895](https://github.com/containers/podman/issues/11895)).
- Fixed a bug where container healthchecks were not properly cleaning up generated systemd services, leading to healthcheck failures after containers were restarted.
- Fixed a bug where the `podman build` command did not properly propagate non-0 exit codes from Buildah when builds failed.
- Fixed a bug where the remote Podman client's `podman build` command could fail to build images when the remote client was run on Windows and the Containerfile contained `COPY` instructions ([#13119](https://github.com/containers/podman/issues/13119)).
- Fixed a bug where the remote Podman client's `--secret` option to the `podman build` command was nonfunctional.
- Fixed a bug where the remote Podman client's `podman build` command would error if given a relative path to a Containerfile ([#12841](https://github.com/containers/podman/issues/12841) and [#12763](https://github.com/containers/podman/issues/12763)).
- Fixed a bug where the `podman generate kube` command would sometimes omit environment variables set in containers from generated YAML.
- Fixed a bug where setting `userns=auto` in `containers.conf` was not respected ([#12615](https://github.com/containers/podman/issues/12615)).
- Fixed a bug where the `podman run` command would fail if the host machine did not have a `/etc/hosts` file ([#12667](https://github.com/containers/podman/issues/12667)).
- Fixed a bug where certain annotations used internally by Podman could be set by images, resulting in `podman inspect` reporting incorrect information ([#12671](https://github.com/containers/podman/issues/12671)).
- Fixed a bug where named volumes would not copy-up after being mounted over an empty directory, then subsequently mounted over a non-empty directory in another container ([#12714](https://github.com/containers/podman/issues/12714)).
- Fixed a bug where the `podman inspect` command on containers was URL-encoding special characters in strings (e.g. healthcheck commands).
- Fixed a bug where the `podman generate kube` command would generate YAML including optional environment variables from secrets and configmaps that are not included ([#12553](https://github.com/containers/podman/issues/12553)).
- Fixed a bug where the `podman pod create` command would ignore the default infra image specified in `containers.conf` ([#12771](https://github.com/containers/podman/issues/12771)).
- Fixed a bug where the `host.containers.internal` entry in `/etc/hosts` was set incorrectly to an inaccessible host IP for `macvlan` networks ([#11351](https://github.com/containers/podman/issues/11351)).
- Fixed a bug where secrets could not be mounted into containers that joined a user namespace (e.g. `--userns=auto`) ([#12779](https://github.com/containers/podman/issues/12779)).
- Fixed a bug where rootless Podman could produce an error about cgroups when containers were created inside existing pods ([#10800](https://github.com/containers/podman/issues/10800)).
- Fixed a bug where Podman could error that a systemd session was not available despite having the cgroup manager set to `cgroupfs` ([#12802](https://github.com/containers/podman/issues/12802)).
- Fixed a bug where the remote Podman client on Windows would ignore environment variables from the `--env` option to `podman create` and `podman run` ([#12056](https://github.com/containers/podman/issues/12056)).
- Fixed a bug where Podman could segfault when an error occurred trying to set up rootless mode.
- Fixed a bug where Podman could segfault when reading an image layer that did not have a creation timestamp set.
- Fixed a bug where, when Podman's storage directories were on an NFS filesystem, Podman would leave some unneeded file descriptors open, causing errors when containers were removed.
- Fixed a bug where, when Podman's storage directories were on an NFS filesystem, cleaning up a container's exec sessions could fail.
- Fixed a bug where Podman commands that operate on a container could give an incorrect error message if given a partial ID that could refer to 2 or more containers ([#12963](https://github.com/containers/podman/issues/12963)).
- Fixed a bug where the `podman stats` command would not show network usage statistics on containers using `slirp4netns` for networking ([#11695](https://github.com/containers/podman/issues/11695)).
- Fixed a bug where the `/dev/shm` mount in the container was not mounted with `nosuid`, `noexec`, and `nodev` mount options.
- Fixed a bug where the `--shm-size` option to `podman create` and `podman run` interpeted human-readable sizes as KB instead of KiB, and GB instead of GiB (such that a kilobyte was interpreted as 1000 bytes, instead of 1024 bytes) ([#13096](https://github.com/containers/podman/issues/13096)).
- Fixed a bug where the `--share=cgroup` option to `podman pod create` controlled whether the pod used a shared Cgroup parent, not whether the Cgroup namespace was shared ([#12765](https://github.com/containers/podman/issues/12765)).
- Fixed a bug where, when a Podman container using the `slirp4netns` network mode was run inside a systemd unit file, systemd could kill the `slirp4netns` process, which is shared between all containers for a given user (thus causing all `slirp4netns`-mode containers for that user to be unable to connect to the internet) ([#13153](https://github.com/containers/podman/issues/13153)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands would not update `/etc/resolv.conf` in the container to add or remove the DNS servers of the networks that were connected or disconnected ([#9603](https://github.com/containers/podman/issues/9603)).

### API
- The Podman remote API version has been bumped to v4.0.0.
- The Compat and Libpod Search endpoints for Images now will never truncate the returned image description. The `noTrunc` query parameter is now ignored as such ([#11894](https://github.com/containers/podman/issues/11894)).
- The Libpod Top endpoints for Containers and Pods now support streaming output using the `stream=true` query parameter ([#12115](https://github.com/containers/podman/issues/12115)).
- The Libpod Create endpoint for Volumes now supports specifying labels for the volume both as `Label` and `Labels` in the provided JSON configuration ([#12102](https://github.com/containers/podman/issues/12102)).
- The Compat Create endpoint for Containers now respects cgroup configuration from `containers.conf` ([#12550](https://github.com/containers/podman/issues/12550)).
- The Compat Create endpoint for Containers now respects user namespace configuration from the `PODMAN_USERNS` environment variable ([#11350](https://github.com/containers/podman/issues/11350)).
- Fixed a bug where the Compat Create endpoint for Containers was ignoring the `HostConfig.StorageOpt` field ([#11016](https://github.com/containers/podman/issues/11016)).
- Fixed a bug where the Compat List endpoint for Containers did not populate the `Mounts` field ([#12734](https://github.com/containers/podman/issues/12734)).
- Fixed a bug where a race condition could cause a crash in the server when the Compat or Libpod Attach endpoints for Containers were invoked ([#12904](https://github.com/containers/podman/issues/12904)).
- Fixed a bug where the Libpod Prune endpoint for Images would return nothing, instead of an empty array, when nothing was pruned.
- Fixed a bug where the Compat List endpoint for Images did not prefix image IDs with `sha256:`.
- Fixed a bug where the Compat Push endpoint for Images would return JSON which did not include the `size` field ([#12468](https://github.com/containers/podman/issues/12468)).
- Fixed a bug where the Compat Load endpoint for Images would refuse to accept input archives that contained more than one image.
- Fixed a bug where the Compat Build endpoint for Images ignored the `quiet` query parameter ([#12566](https://github.com/containers/podman/issues/12566)).
- Fixed a bug where the Compat Build endpoint for Images did not include `aux` JSON (which included the ID of built images) in returned output ([#12063](https://github.com/containers/podman/issues/12063)).
- Fixed a bug where the Compat Build endpoint for Images did not set the correct `Content-Type` in its responses ([#13148](https://github.com/containers/podman/issues/13148)).
- Fixed a bug where the Compat and Libpod List endpoints for Networks would sometimes not return networks created on the server by the Podman CLI after the API server had been started ([#11828](https://github.com/containers/podman/issues/11828)).
- Fixed a bug where the Compat Inspect endpoint for Networks did not include the subnet CIDR in the returned IPv4 and IPv6 addresses.
- Fixed a bug where the Compat Events endpoint did not properly set the Action field of `Died` events for containers to `die` (previously, `died` was used; this was incompatible with Docker's output).
- Fixed a bug where the Compat Info endpoint did not properly populate information on configured registries.
- Fixed a bug where the Compat Events endpoint did not properly set the exit code of the container in the `exitCode` field in `Died` events for containers.
- Fixed a bug where the Compat Events endpoint did not properly populate the `TimeNano` field.
- Numerous small changes have been made to ensure that the API matches its Swagger documentation

### Misc
- The Windows installer MSI distributed through Github releases no longer supports 32-bit systems, as Podman is built only for 64-bit machines.
- Updated Buildah to v1.24.0
- Updated the containers/image library to v5.19.0
- Updated the containers/storage library to v1.38.1
- Updated the containers/common library to v0.47.1
- Updated the containers/psgo library to v1.7.2

## 3.4.7
### Security
- This release addresses CVE-2022-1227, where running `podman top` on a container made from a maliciously-crafted image and using a user namespace could allow for code execution in the host context.

## 3.4.6
### Security
- This release addresses CVE-2022-27191, where an attacker could potentially cause crashes in remote Podman by using incorrect SSH ciphers.

## 3.4.5
### Security
- This release addresses CVE-2022-27649, where Podman would set excess inheritable capabilities for processes in containers.

### Bugfixes
- Fixed a bug where the `podman images` command could, under some circumstances, take an excessive amount of time to list images ([#11997](https://github.com/containers/podman/issues/11997)).

### Misc
- Updates the containers/common library to v0.44.5

## 3.4.4
### Bugfixes
- Fixed a bug where the `podman exec` command would, under some circumstances, print a warning message about failing to move `conmon` to the appropriate cgroup ([#12535](https://github.com/containers/podman/issues/12535)).
- Fixed a bug where named volumes created as part of container creation (e.g. `podman run --volume avolume:/a/mountpoint` or similar) would be mounted with incorrect permissions ([#12523](https://github.com/containers/podman/issues/12523)).
- Fixed a bug where the `podman-remote create` and `podman-remote run` commands did not properly handle the `--entrypoint=""` option (to clear the container's entrypoint) ([#12521](https://github.com/containers/podman/issues/12521)).

## 3.4.3
### Security
- This release addresses CVE-2021-4024, where the `podman machine` command opened the `gvproxy` API (used to forward ports to `podman machine` VMs) to the public internet on port 7777.
- This release addresses CVE-2021-41190, where incomplete specification of behavior regarding image manifests could lead to inconsistent decoding on different clients.

### Features
- The `--secret type=mount` option to `podman create` and `podman run` supports a new option, `target=`, which specifies where in the container the secret will be mounted ([#12287](https://github.com/containers/podman/issues/12287)).

### Bugfixes
- Fixed a bug where rootless Podman would occasionally print warning messages about failing to move the pause process to a new cgroup ([#12065](https://github.com/containers/podman/issues/12065)).
- Fixed a bug where the `podman run` and `podman create` commands would, when pulling images, still require TLS even with registries set to Insecure via config file ([#11933](https://github.com/containers/podman/issues/11933)).
- Fixed a bug where the `podman generate systemd` command generated units that depended on `multi-user.target`, which has been removed from some distributions ([#12438](https://github.com/containers/podman/issues/12438)).
- Fixed a bug where Podman could not run containers with images that had `/etc/` as a symlink ([#12189](https://github.com/containers/podman/issues/12189)).
- Fixed a bug where the `podman logs -f` command would, when using the `journald` logs backend, exit immediately if the container had previously been restarted ([#12263](https://github.com/containers/podman/issues/12263)).
- Fixed a bug where, in containers on VMs created by `podman machine`, the `host.containers.internal` name pointed to the VM, not the host system ([#11642](https://github.com/containers/podman/issues/11642)).
- Fixed a bug where containers and pods created by the `podman play kube` command in VMs managed by `podman machine` would not automatically forward ports from the host machine ([#12248](https://github.com/containers/podman/issues/12248)).
- Fixed a bug where `podman machine init` would fail on OS X when GNU Coreutils was installed ([#12329](https://github.com/containers/podman/issues/12329)).
- Fixed a bug where `podman machine start` would exit before SSH on the started VM was accepting connections ([#11532](https://github.com/containers/podman/issues/11532)).
- Fixed a bug where the `podman run` command with signal proxying (`--sig-proxy`) enabled could print an error if it attempted to send a signal to a container that had just exited ([#8086](https://github.com/containers/podman/issues/8086)).
- Fixed a bug where the `podman stats` command would not return correct information for containers running Systemd as PID1 ([#12400](https://github.com/containers/podman/issues/12400)).
- Fixed a bug where the `podman image save` command would fail on OS X when writing the image to STDOUT ([#12402](https://github.com/containers/podman/issues/12402)).
- Fixed a bug where the `podman ps` command did not properly handle PS arguments which contained whitespace ([#12452](https://github.com/containers/podman/issues/12452)).
- Fixed a bug where the `podman-remote wait` command could fail to detect that the container exited and return an error under some circumstances ([#12457](https://github.com/containers/podman/issues/12457)).
- Fixed a bug where the Windows MSI installer for `podman-remote` would break the PATH environment variable by adding an extra `"` ([#11416](https://github.com/containers/podman/issues/11416)).

### API
- Updated the containers/image library to v5.17.0
- The Libpod Play Kube endpoint now also accepts `ConfigMap` YAML as part of its payload, and will use provided any `ConfigMap` to configure provided pods and services.
- Fixed a bug where the Compat Create endpoint for Containers would not always create the container's working directory if it did not exist ([#11842](https://github.com/containers/podman/issues/11842)).
- Fixed a bug where the Compat Create endpoint for Containers returned an incorrect error message with 404 errors when the requested image was not found ([#12315](https://github.com/containers/podman/pull/12315)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the `HostConfig.Mounts` field ([#12419](https://github.com/containers/podman/issues/12419)).
- Fixed a bug where the Compat Archive endpoint for Containers did not properly report errors when the operation failed ([#12420](https://github.com/containers/podman/issues/12420)).
- Fixed a bug where the Compat Build endpoint for Images ignored the `layers` query parameter (for caching intermediate layers from the build) ([#12378](https://github.com/containers/podman/issues/12378)).
- Fixed a bug where the Compat Build endpoint for Images did not report errors in a manner compatible with Docker ([#12392](https://github.com/containers/podman/issues/12392)).
- Fixed a bug where the Compat Build endpoint for Images would fail to build if the context directory was a symlink ([#12409](https://github.com/containers/podman/issues/12409)).
- Fixed a bug where the Compat List endpoint for Images included manifest lists (and not just images) in returned results ([#12453](https://github.com/containers/podman/issues/12453)).

### Misc
- Updated the containers/image library to v5.17.0
- Updated the containers/storage library to v1.37.0
- Podman now builds by default with cgo enabled on OS X, resolving some issues with SSH ([#10737](https://github.com/containers/podman/issues/10737)).

## 3.4.2
### Bugfixes
- Fixed a bug where `podman tag` could not tag manifest lists ([#12046](https://github.com/containers/podman/issues/12046)).
- Fixed a bug where built-in volumes specified by images would not be created correctly under some circumstances.
- Fixed a bug where, when using Podman Machine on OS X, containers in pods did not have working port forwarding from the host ([#12207](https://github.com/containers/podman/issues/12207)).
- Fixed a bug where the `podman network reload` command command on containers using the `slirp4netns` network mode and the `rootlessport` port forwarding driver would make an unnecessary attempt to restart `rootlessport` on containers that did not forward ports.
- Fixed a bug where the `podman generate kube` command would generate YAML including some unnecessary (set to default) fields (e.g. empty SELinux and DNS configuration blocks, and the `privileged` flag when set to false) ([#11995](https://github.com/containers/podman/issues/11995)).
- Fixed a bug where the `podman pod rm` command could, if interrupted at the right moment, leave a reference to an already-removed infra container behind ([#12034](https://github.com/containers/podman/issues/12034)).
- Fixed a bug where the `podman pod rm` command would not remove pods with more than one container if all containers save for the infra container were stopped unless `--force` was specified ([#11713](https://github.com/containers/podman/issues/11713)).
- Fixed a bug where the `--memory` flag to `podman run` and `podman create` did not accept a limit of 0 (which should specify unlimited memory) ([#12002](https://github.com/containers/podman/issues/12002)).
- Fixed a bug where the remote Podman client's `podman build` command could attempt to build a Dockerfile in the working directory of the `podman system service` instance instead of the Dockerfile specified by the user ([#12054](https://github.com/containers/podman/issues/12054)).
- Fixed a bug where the `podman logs --tail` command could function improperly (printing more output than requested) when the `journald` log driver was used.
- Fixed a bug where containers run using the `slirp4netns` network mode with IPv6 enabled would not have IPv6 connectivity until several seconds after they started ([#11062](https://github.com/containers/podman/issues/11062)).
- Fixed a bug where some Podman commands could cause an extra `dbus-daemon` process to be created ([#9727](https://github.com/containers/podman/issues/9727)).
- Fixed a bug where rootless Podman would sometimes print warnings about a failure to move the pause process into a given CGroup ([#12065](https://github.com/containers/podman/issues/12065)).
- Fixed a bug where the `checkpointed` field in `podman inspect` on a container was not set to false after a container was restored.
- Fixed a bug where the `podman system service` command would print overly-verbose logs about request IDs ([#12181](https://github.com/containers/podman/issues/12181)).
- Fixed a bug where Podman could, when creating a new container without a name explicitly specified by the user, sometimes use an auto-generated name already in use by another container if multiple containers were being created in parallel ([#11735](https://github.com/containers/podman/issues/11735)).

## 3.4.1
### Bugfixes
- Fixed a bug where `podman machine init` could, under some circumstances, create invalid machine configurations which could not be started ([#11824](https://github.com/containers/podman/issues/11824)).
- Fixed a bug where the `podman machine list` command would not properly populate some output fields.
- Fixed a bug where `podman machine rm` could leave dangling sockets from the removed machine ([#11393](https://github.com/containers/podman/issues/11393)).
- Fixed a bug where `podman run --pids-limit=-1` was not supported (it now sets the PID limit in the container to unlimited) ([#11782](https://github.com/containers/podman/issues/11782)).
- Fixed a bug where `podman run` and `podman attach` could throw errors about a closed network connection when STDIN was closed by the client ([#11856](https://github.com/containers/podman/issues/11856)).
- Fixed a bug where the `podman stop` command could fail when run on a container that had another `podman stop` command run on it previously.
- Fixed a bug where the `--sync` flag to `podman ps` was nonfunctional.
- Fixed a bug where the Windows and OS X remote clients' `podman stats` command would fail ([#11909](https://github.com/containers/podman/issues/11909)).
- Fixed a bug where the `podman play kube` command did not properly handle environment variables whose values contained an `=` ([#11891](https://github.com/containers/podman/issues/11891)).
- Fixed a bug where the `podman generate kube` command could generate invalid annotations when run on containers with volumes that use SELinux relabelling (`:z` or `:Z`) ([#11929](https://github.com/containers/podman/issues/11929)).
- Fixed a bug where the `podman generate kube` command would generate YAML including some unnecessary (set to default) fields (e.g. user and group, entrypoint, default protocol for forwarded ports) ([#11914](https://github.com/containers/podman/issues/11914), [#11915](https://github.com/containers/podman/issues/11915), and [#11965](https://github.com/containers/podman/issues/11965)).
- Fixed a bug where the `podman generate kube` command could, under some circumstances, generate YAML including an invalid `targetPort` field for forwarded ports ([#11930](https://github.com/containers/podman/issues/11930)).
- Fixed a bug where rootless Podman's `podman info` command could, under some circumstances, not read available CGroup controllers ([#11931](https://github.com/containers/podman/issues/11931)).
- Fixed a bug where `podman container checkpoint --export` would fail to checkpoint any container created with `--log-driver=none` ([#11974](https://github.com/containers/podman/issues/11974)).

### API
- Fixed a bug where the Compat Create endpoint for Containers could panic when no options were passed to a bind mount of tmpfs ([#11961](https://github.com/containers/podman/issues/11961)).

## 3.4.0
### Features
- Pods now support init containers! Init containers are containers which run before the rest of the pod starts. There are two types of init containers: "always", which always run before the pod is started, and "once", which only run the first time the pod starts and are subsequently removed. They can be added using the `podman create` command's `--init-ctr` option.
- Support for init containers has also been added to `podman play kube` and `podman generate kube` - init containers contained in Kubernetes YAML will be created as Podman init containers, and YAML generated by Podman will include any init containers created.
- The `podman play kube` command now supports building images. If the `--build` option is given and a directory with the name of the specified image exists in the current working directory and contains a valid Containerfile or Dockerfile, the image will be built and used for the container.
- The `podman play kube` command now supports a new option, `--teardown`, which removes any pods and containers created by the given Kubernetes YAML.
- The `podman generate kube` command now generates annotations for SELinux mount options on volume (`:z` and `:Z`) that are respected by the `podman play kube` command.
- A new command has been added, `podman pod logs`, to return logs for all containers in a pod at the same time.
- Two new commands have been added, `podman volume export` (to export a volume to a tar file) and `podman volume import`) (to populate a volume from a given tar file).
- The `podman auto-update` command now supports simple rollbacks. If a container fails to start after an automatic update, it will be rolled back to the previous image and restarted again.
- Pods now share their user namespace by default, and the `podman pod create` command now supports the `--userns` option. This allows rootless pods to be created with the `--userns=keep-id` option.
- The `podman pod ps` command now supports a new filter with its `--filter` option, `until`, which returns pods created before a given timestamp.
- The `podman image scp` command has been added. This command allows images to be transferred between different hosts.
- The `podman stats` command supports a new option, `--interval`, to specify the amount of time before the information is refreshed.
- The `podman inspect` command now includes ports exposed (but not published) by containers (e.g. ports from `--expose` when `--publish-all` is not specified).
- The `podman inspect` command now has a new boolean value, `Checkpointed`, which indicates that a container was stopped as a result of a `podman container checkpoint` operation.
- Volumes created by `podman volume create` now support setting quotas when run atop XFS. The `size` and `inode` options allow the maximum size and maximum number of inodes consumed by a volume to be limited.
- The `podman info` command now outputs information on what log drivers, network drivers, and volume plugins are available for use ([#11265](https://github.com/containers/podman/issues/11265)).
- The `podman info` command now outputs the current log driver in use, and the variant and codename of the distribution in use.
- The parameters of the VM created by `podman machine init` (amount of disk space, memory, CPUs) can now be set in `containers.conf`.
- The `podman machine ls` command now shows additional information (CPUs, memory, disk size) about VMs managed by `podman machine`.
- The `podman ps` command now includes healthcheck status in container state for containers that have healthchecks ([#11527](https://github.com/containers/podman/issues/11527)).

### Changes
- The `podman build` command has a new alias, `podman buildx`, to improve compatibility with Docker. We have already added support for many `docker buildx` flags to `podman build` and aim to continue to do so.
- Cases where Podman is run without a user session or a writable temporary files directory will now produce better error messages.
- The default log driver has been changed from `file` to `journald`. The `file` driver did not properly support log rotation, so this should lead to a better experience. If journald is not available on the system, Podman will automatically revert to the `file`.
- Podman no longer depends on `ip` for removing networks ([#11403](https://github.com/containers/podman/issues/11403)).
- The deprecated `--macvlan` flag to `podman network create` now warns when it is used. It will be removed entirely in the Podman 4.0 release.
- The `podman machine start` command now prints a message when the VM is successfully started.
- The `podman stats` command can now be used on containers that are paused.
- The `podman unshare` command will now return the exit code of the command that was run in the user namespace (assuming the command was successfully run).
- Successful healthchecks will no longer add a `healthy` line to the system log to reduce log spam.
- As a temporary workaround for a lack of shortname prompts in the Podman remote client, VMs created by `podman machine` now default to only using the `docker.io` registry.

### Bugfixes
- Fixed a bug where whitespace in the definition of sysctls (particularly default sysctls specified in `containers.conf`) would cause them to be parsed incorrectly.
- Fixed a bug where the Windows remote client improperly validated volume paths ([#10900](https://github.com/containers/podman/issues/10900)).
- Fixed a bug where the first line of logs from a container run with the `journald` log driver could be skipped.
- Fixed a bug where images created by `podman commit` did not include ports exposed by the container.
- Fixed a bug where the `podman auto-update` command would ignore the `io.containers.autoupdate.authfile` label when pulling images ([#11171](https://github.com/containers/podman/issues/11171)).
- Fixed a bug where the `--workdir` option to `podman create` and `podman run` could not be set to a directory where a volume was mounted ([#11352](https://github.com/containers/podman/issues/11352)).
- Fixed a bug where systemd socket-activation did not properly work with systemd-managed Podman containers ([#10443](https://github.com/containers/podman/issues/10443)).
- Fixed a bug where environment variable secrets added to a container were not available to exec sessions launched in the container.
- Fixed a bug where rootless containers could fail to start the `rootlessport` port-forwarding service when `XDG_RUNTIME_DIR` was set to a long path.
- Fixed a bug where arguments to the `--systemd` option to `podman create` and `podman run` were case-sensitive ([#11387](https://github.com/containers/podman/issues/11387)).
- Fixed a bug where the `podman manifest rm` command would also remove images referenced by the manifest, not just the manifest itself ([#11344](https://github.com/containers/podman/issues/11344)).
- Fixed a bug where the Podman remote client on OS X would not function properly if the `TMPDIR` environment variable was not set ([#11418](https://github.com/containers/podman/issues/11418)).
- Fixed a bug where the `/etc/hosts` file was not guaranteed to contain an entry for `localhost` (this is still not guaranteed if `--net=host` is used; such containers will exactly match the host's `/etc/hosts`) ([#11411](https://github.com/containers/podman/issues/11411)).
- Fixed a bug where the `podman machine start` command could print warnings about unsupported CPU features ([#11421](https://github.com/containers/podman/issues/11421)).
- Fixed a bug where the `podman info` command could segfault when accessing cgroup information.
- Fixed a bug where the `podman logs -f` command could hang when a container exited ([#11461](https://github.com/containers/podman/issues/11461)).
- Fixed a bug where the `podman generate systemd` command could not be used on containers that specified a restart policy ([#11438](https://github.com/containers/podman/issues/11438)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build containers if the UID and GID on the client were higher than 65536 ([#11474](https://github.com/containers/podman/issues/11474)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build containers if the context directory was a symlink ([#11732](https://github.com/containers/podman/issues/11732)).
- Fixed a bug where the `--network` flag to `podman play kube` was not properly parsed when a non-bridge network configuration was specified.
- Fixed a bug where the `podman inspect` command could error when the container being inspected was removed as it was being inspected ([#11392](https://github.com/containers/podman/issues/11392)).
- Fixed a bug where the `podman play kube` command ignored the default pod infra image specified in `containers.conf`.
- Fixed a bug where the `--format` option to `podman inspect` was nonfunctional under some circumstances ([#8785](https://github.com/containers/podman/issues/8785)).
- Fixed a bug where the remote Podman client's `podman run` and `podman exec` commands could skip a byte of output every 8192 bytes ([#11496](https://github.com/containers/podman/issues/11496)).
- Fixed a bug where the `podman stats` command would print nonsensical results if the container restarted while it was running ([#11469](https://github.com/containers/podman/issues/11469)).
- Fixed a bug where the remote Podman client would error when STDOUT was redirected on a Windows client ([#11444](https://github.com/containers/podman/issues/11444)).
- Fixed a bug where the `podman run` command could return 0 when the application in the container exited with 125 ([#11540](https://github.com/containers/podman/issues/11540)).
- Fixed a bug where containers with `--restart=always` set using the rootlessport port-forwarding service could not be restarted automatically.
- Fixed a bug where the `--cgroups=split` option to `podman create` and `podman run` was silently discarded if the container was part of a pod.
- Fixed a bug where the `podman container runlabel` command could fail if the image name given included a tag.
- Fixed a bug where Podman could add an extra `127.0.0.1` entry to `/etc/hosts` under some circumstances ([#11596](https://github.com/containers/podman/issues/11596)).
- Fixed a bug where the remote Podman client's `podman untag` command did not properly handle tags including a digest ([#11557](https://github.com/containers/podman/issues/11557)).
- Fixed a bug where the `--format` option to `podman ps` did not properly support the `table` argument for tabular output.
- Fixed a bug where the `--filter` option to `podman ps` did not properly handle filtering by healthcheck status ([#11687](https://github.com/containers/podman/issues/11687)).
- Fixed a bug where the `podman run` and `podman start --attach` commands could race when retrieving the exit code of a container that had already been removed resulting in an error (e.g. by an external `podman rm -f`) ([#11633](https://github.com/containers/podman/issues/11633)).
- Fixed a bug where the `podman generate kube` command would add default environment variables to generated YAML.
- Fixed a bug where the `podman generate kube` command would add the default CMD from the image to generated YAML ([#11672](https://github.com/containers/podman/issues/11672)).
- Fixed a bug where the `podman rm --storage` command could fail to remove containers under some circumstances ([#11207](https://github.com/containers/podman/issues/11207)).
- Fixed a bug where the `podman machine ssh` command could fail when run on Linux ([#11731](https://github.com/containers/podman/issues/11731)).
- Fixed a bug where the `podman stop` command would error when used on a container that was already stopped ([#11740](https://github.com/containers/podman/issues/11740)).
- Fixed a bug where renaming a container in a pod using the `podman rename` command, then removing the pod using `podman pod rm`, could cause Podman to believe the new name of the container was permanently in use, despite the container being removed ([#11750](https://github.com/containers/podman/issues/11750)).

### API
- The Libpod Pull endpoint for Images now has a new query parameter, `quiet`, which (when set to true) suppresses image pull progress reports ([#10612](https://github.com/containers/podman/issues/10612)).
- The Compat Events endpoint now includes several deprecated fields from the Docker v1.21 API for improved compatibility with older clients.
- The Compat List and Inspect endpoints for Images now prefix image IDs with `sha256:` for improved Docker compatibility ([#11623](https://github.com/containers/podman/issues/11623)).
- The Compat Create endpoint for Containers now properly sets defaults for healthcheck-related fields ([#11225](https://github.com/containers/podman/issues/11225)).
- The Compat Create endpoint for Containers now supports volume options provided by the `Mounts` field ([#10831](https://github.com/containers/podman/issues/10831)).
- The Compat List endpoint for Secrets now supports a new query parameter, `filter`, which allows returned results to be filtered.
- The Compat Auth endpoint now returns the correct response code (500 instead of 400) when logging into a registry fails.
- The Version endpoint now includes information about the OCI runtime and Conmon in use ([#11227](https://github.com/containers/podman/issues/11227)).
- Fixed a bug where the X-Registry-Config header was not properly handled, leading to errors when pulling images ([#11235](https://github.com/containers/podman/issues/11235)).
- Fixed a bug where invalid query parameters could cause a null pointer dereference when creating error messages.
- Logging of API requests and responses at trace level has been greatly improved, including the addition of an X-Reference-Id header to correlate requests and responses ([#10053](https://github.com/containers/podman/issues/10053)).

### Misc
- Updated Buildah to v1.23.0
- Updated the containers/storage library to v1.36.0
- Updated the containers/image library to v5.16.0
- Updated the containers/common library to v0.44.0

## 3.3.1
### Bugfixes
- Fixed a bug where unit files created by `podman generate systemd` could not cleanup shut down containers when stopped by `systemctl stop` ([#11304](https://github.com/containers/podman/issues/11304)).
- Fixed a bug where `podman machine` commands would not properly locate the `gvproxy` binary in some circumstances.
- Fixed a bug where containers created as part of a pod using the `--pod-id-file` option would not join the pod's network namespace ([#11303](https://github.com/containers/podman/issues/11303)).
- Fixed a bug where Podman, when using the systemd cgroups driver, could sometimes leak dbus sessions.
- Fixed a bug where the `until` filter to `podman logs` and `podman events` was improperly handled, requiring input to be negated ([#11158](https://github.com/containers/podman/issues/11158)).
- Fixed a bug where rootless containers using CNI networking run on systems using `systemd-resolved` for DNS would fail to start if resolved symlinked `/etc/resolv.conf` to an absolute path ([#11358](https://github.com/containers/podman/issues/11358)).

### API
- A large number of potential file descriptor leaks from improperly closing client connections have been fixed.

## 3.3.0
### Features
- Containers inside VMs created by `podman machine` will now automatically handle port forwarding - containers in `podman machine` VMs that publish ports via `--publish` or `--publish-all` will have these ports not just forwarded on the VM, but also on the host system.
- The `podman play kube` command's `--network` option now accepts advanced network options (e.g. `--network slirp4netns:port_handler=slirp4netns`) ([#10807](https://github.com/containers/podman/issues/10807)).
- The `podman play kube` command now supports Kubernetes liveness probes, which will be created as Podman healthchecks.
- Podman now provides a systemd unit, `podman-restart.service`, which, when enabled, will restart all containers that were started with `--restart=always` after the system reboots.
- Rootless Podman can now be configured to use CNI networking by default by using the `rootless_networking` option in `containers.conf`.
- Images can now be pulled using `image:tag@digest` syntax (e.g. `podman pull fedora:34@sha256:1b0d4ddd99b1a8c8a80e885aafe6034c95f266da44ead992aab388e6aa91611a`) ([#6721](https://github.com/containers/podman/issues/6721)).
- The `podman container checkpoint` and `podman container restore` commands can now be used to checkpoint containers that are in pods, and restore those containers into pods.
- The `podman container restore` command now features a new option, `--publish`, to change the ports that are forwarded to a container that is being restored from an exported checkpoint.
- The `podman container checkpoint` command now features a new option, `--compress`, to specify the compression algorithm that will be used on the generated checkpoint.
- The `podman pull` command can now pull multiple images at once (e.g. `podman pull fedora:34 ubi8:latest` will pull both specified images).
- THe `podman cp` command can now copy files from one container into another directly (e.g. `podman cp containera:/etc/hosts containerb:/etc/`) ([#7370](https://github.com/containers/podman/issues/7370)).
- The `podman cp` command now supports a new option, `--archive`, which controls whether copied files will be chown'd to the UID and GID of the user of the destination container.
- The `podman stats` command now provides two additional metrics: Average CPU, and CPU time.
- The `podman pod create` command supports a new flag, `--pid`, to specify the PID namespace of the pod. If specified, containers that join the pod will automatically share its PID namespace.
- The `podman pod create` command supports a new flag, `--infra-name`, which allows the name of the pod's infra container to be set ([#10794](https://github.com/containers/podman/issues/10794)).
- The `podman auto-update` command has had its output reformatted - it is now much clearer what images were pulled and what containers were updated.
- The `podman auto-update` command now supports a new option, `--dry-run`, which reports what would be updated but does not actually perform the update ([#9949](https://github.com/containers/podman/issues/9949)).
- The `podman build` command now supports a new option, `--secret`, to mount secrets into build containers.
- The `podman manifest remove` command now has a new alias, `podman manifest rm`.
- The `podman login` command now supports a new option, `--verbose`, to print detailed information about where the credentials entered were stored.
- The `podman events` command now supports a new event, `exec_died`, which is produced when an exec session exits, and includes the exit code of the exec session.
- The `podman system connection add` command now supports adding connections that connect using the `tcp://` and `unix://` URL schemes.
- The `podman system connection list` command now supports a new flag, `--format`, to determine how the output is printed.
- The `podman volume prune` and `podman volume ls` commands' `--filter` option now support a new filter, `until`, that matches volumes created before a certain time ([#10579](https://github.com/containers/podman/issues/10579)).
- The `podman ps --filter` option's `network` filter now accepts a new value: `container:`, which matches containers that share a network namespace with a specific container ([#10361](https://github.com/containers/podman/issues/10361)).
- The `podman diff` command can now accept two arguments, allowing two images or two containers to be specified; the diff between the two will be printed ([#10649](https://github.com/containers/podman/issues/10649)).
- Podman can now optionally copy-up content from containers into volumes mounted into those containers earlier (at creation time, instead of at runtime) via the `prepare_on_create` option in `containers.conf` ([#10262](https://github.com/containers/podman/issues/10262)).
- A new option, `--gpus`, has been added to `podman create` and `podman run` as a no-op for better compatibility with Docker. If the nvidia-container-runtime package is installed, GPUs should be automatically added to containers without using the flag.
- If an invalid subcommand is provided, similar commands to try will now be suggested in the error message.

### Changes
- The `podman system reset` command now removes non-Podman (e.g. Buildah and CRI-O) containers as well.
- The new port forwarding offered by `podman machine` requires [gvproxy](https://github.com/containers/gvisor-tap-vsock) in order to function.
- Podman will now automatically create the default CNI network if it does not exist, for both root and rootless users. This will only be done once per user - if the network is subsequently removed, it will not be recreated.
- The `install.cni` makefile option has been removed. It is no longer required to distribute the default `87-podman.conflist` CNI configuration file, as Podman will now automatically create it.
- The `--root` option to Podman will not automatically clear all default storage options when set. Storage options can be set manually using `--storage-opt` ([#10393](https://github.com/containers/podman/issues/10393)).
- The output of `podman system connection list` is now deterministic, with connections being sorted alpabetically by their name.
- The auto-update service (`podman-auto-update.service`) has had its default timer adjusted so it now starts at a random time up to 15 minutes after midnight, to help prevent system congestion from numerous daily services run at once.
- Systemd unit files generated by `podman generate systemd` now depend on `network-online.target` by default ([#10655](https://github.com/containers/podman/issues/10655)).
- Systemd unit files generated by `podman generate systemd` now use `Type=notify` by default, instead of using PID files.
- The `podman info` command's logic for detecting package versions on Gentoo has been improved, and should be significantly faster.

### Bugfixes
- Fixed a bug where the `podman play kube` command did not perform SELinux relabelling of volumes specified with a `mountPath` that included the `:z` or `:Z` options ([#9371](https://github.com/containers/podman/issues/9371)).
- Fixed a bug where the `podman play kube` command would ignore the `USER` and `EXPOSE` directives in images ([#9609](https://github.com/containers/podman/issues/9609)).
- Fixed a bug where the `podman play kube` command would only accept lowercase pull policies.
- Fixed a bug where named volumes mounted into containers with the `:z` or `:Z` options were not appropriately relabelled for access from the container ([#10273](https://github.com/containers/podman/issues/10273)).
- Fixed a bug where the `podman logs -f` command, with the `journald` log driver, could sometimes fail to pick up the last line of output from a container ([#10323](https://github.com/containers/podman/issues/10323)).
- Fixed a bug where running `podman rm` on a container created with the `--rm` option would occasionally emit an error message saying the container failed to be removed, when it was successfully removed.
- Fixed a bug where starting a Podman container would segfault if the `LISTEN_PID` and `LISTEN_FDS` environment variables were set, but `LISTEN_FDNAMES` was not ([#10435](https://github.com/containers/podman/issues/10435)).
- Fixed a bug where exec sessions in containers were sometimes not cleaned up when run without `-d` and when the associated `podman exec` process was killed before completion.
- Fixed a bug where `podman system service` could, when run in a systemd unit file with sdnotify in use, drop some connections when it was starting up.
- Fixed a bug where containers run using the REST API using the `slirp4netns` network mode would leave zombie processes that were not cleaned up until `podman system service` exited ([#9777](https://github.com/containers/podman/issues/9777)).
- Fixed a bug where the `podman system service` command would leave zombie processes after its initial launch that were not cleaned up until it exited ([#10575](https://github.com/containers/podman/issues/10575)).
- Fixed a bug where VMs created by `podman machine` could not be started after the host system restarted ([#10824](https://github.com/containers/podman/issues/10824)).
- Fixed a bug where the `podman pod ps` command would not show headers for optional information (e.g. container names when the `--ctr-names` option was given).
- Fixed a bug where the remote Podman client's `podman create` and `podman run` commands would ignore timezone configuration from the server's `containers.conf` file ([#11124](https://github.com/containers/podman/issues/11124)).
- Fixed a bug where the remote Podman client's `podman build` command would only respect `.containerignore` and not `.dockerignore` files (when both are present, `.containerignore` will be preferred) ([#10907](https://github.com/containers/podman/issues/10907)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to send the Dockerfile being built to the server when it was excluded by the `.dockerignore` file, resulting in an error ([#9867](https://github.com/containers/podman/issues/9867)).
- Fixed a bug where the remote Podman client's `podman build` command could unexpectedly stop streaming the output of the build ([#10154](https://github.com/containers/podman/issues/10154)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build when run on Windows ([#11259](https://github.com/containers/podman/issues/11259)).
- Fixed a bug where the `podman manifest create` command accepted at most two arguments (an arbitrary number of images are allowed as arguments, which will be added to the manifest).
- Fixed a bug where named volumes would not be properly chowned to the UID and GID of the directory they were mounted over when first mounted into a container ([#10776](https://github.com/containers/podman/issues/10776)).
- Fixed a bug where named volumes created using a volume plugin would be removed from Podman, even if the plugin reported a failure to remove the volume ([#11214](https://github.com/containers/podman/issues/11214)).
- Fixed a bug where the remote Podman client's `podman exec -i` command would hang when input was provided via shell redirection (e.g. `podman --remote exec -i foo cat <<<"hello"`) ([#7360](https://github.com/containers/podman/issues/7360)).
- Fixed a bug where containers created with `--rm` were not immediately removed after being started by `podman start` if they failed to start ([#10935](https://github.com/containers/podman/issues/10935)).
- Fixed a bug where the `--storage-opt` flag to `podman create` and `podman run` was nonfunctional ([#10264](https://github.com/containers/podman/issues/10264)).
- Fixed a bug where the `--device-cgroup-rule` option to `podman create` and `podman run` was nonfunctional ([#10302](https://github.com/containers/podman/issues/10302)).
- Fixed a bug where the `--tls-verify` option to `podman manifest push` was nonfunctional.
- Fixed a bug where the `podman import` command could, in some circumstances, produce empty images ([#10994](https://github.com/containers/podman/issues/10994)).
- Fixed a bug where images pulled using the `docker-daemon:` transport had the wrong registry (`localhost` instead of `docker.io/library`) ([#10998](https://github.com/containers/podman/issues/10998)).
- Fixed a bug where operations that pruned images (`podman image prune` and `podman system prune`) would prune untagged images with children ([#10832](https://github.com/containers/podman/issues/10832)).
- Fixed a bug where dual-stack networks created by `podman network create` did not properly auto-assign an IPv4 subnet when one was not explicitly specified ([#11032](https://github.com/containers/podman/issues/11032)).
- Fixed a bug where port forwarding using the `rootlessport` port forwarder would break when a network was disconnected and then reconnected ([#10052](https://github.com/containers/podman/issues/10052)).
- Fixed a bug where Podman would ignore user-specified SELinux policies for containers using the Kata OCI runtime, or containers using systemd as PID 1 ([#11100](https://github.com/containers/podman/issues/11100)).
- Fixed a bug where Podman containers created using `--net=host` would add an entry to `/etc/hosts` for the container's hostname pointing to `127.0.1.1` ([#10319](https://github.com/containers/podman/issues/10319)).
- Fixed a bug where the `podman unpause --all` command would throw an error for every container that was not paused ([#11098](https://github.com/containers/podman/issues/11098)).
- Fixed a bug where timestamps for the `since` and `until` filters using Unix timestamps with a nanoseconds portion could not be parsed ([#11131](https://github.com/containers/podman/issues/11131)).
- Fixed a bug where the `podman info` command would sometimes print the wrong path for the `slirp4netns` binary.
- Fixed a bug where rootless Podman containers joined to a CNI network would not have functional DNS when the host used systemd-resolved without the resolved stub resolver being enabled ([#11222](https://github.com/containers/podman/issues/11222)).
- Fixed a bug where `podman network connect` and `podman network disconnect` of rootless containers could sometimes break port forwarding to the container ([#11248](https://github.com/containers/podman/issues/11248)).
- Fixed a bug where joining a container to a CNI network by ID and adding network aliases to this network would cause the container to fail to start ([#11285](https://github.com/containers/podman/issues/11285)).

### API
- Fixed a bug where the Compat List endpoint for Containers included healthcheck information for all containers, even those that did not have a configured healthcheck.
- Fixed a bug where the Compat Create endpoint for Containers would fail to create containers with the `NetworkMode` parameter set to `default` ([#10569](https://github.com/containers/podman/issues/10569)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle healthcheck commands ([#10617](https://github.com/containers/podman/issues/10617)).
- Fixed a bug where the Compat Wait endpoint for Containers would always send an empty string error message when no error occurred.
- Fixed a bug where the Libpod Stats endpoint for Containers would not error when run on rootless containers on cgroups v1 systems (nonsensical results would be returned, as this configuration cannot be supportable).
- Fixed a bug where the Compat List endpoint for Images omitted the `ContainerConfig` field ([#10795](https://github.com/containers/podman/issues/10795)).
- Fixed a bug where the Compat Build endpoint for Images was too strict when validating the `Content-Type` header, rejecting content that Docker would have accepted ([#11022](https://github.com/containers/podman/issues/11012)).
- Fixed a bug where the Compat Pull endpoint for Images could fail, but return a 200 status code, if an image name that could not be parsed was provided.
- Fixed a bug where the Compat Pull endpoint for Images would continue to pull images after the client disconnected.
- Fixed a bug where the Compat List endpoint for Networks would fail for non-bridge (e.g. macvlan) networks ([#10266](https://github.com/containers/podman/issues/10266)).
- Fixed a bug where the Libpod List endpoint for Networks would return nil, instead of an empty list, when no networks were present ([#10495](https://github.com/containers/podman/issues/10495)).
- The Compat and Libpod Logs endpoints for Containers now support the `until` query parameter ([#10859](https://github.com/containers/podman/issues/10859)).
- The Compat Import endpoint for Images now supports the `platform`, `message`, and `repo` query parameters.
- The Compat Pull endpoint for Images now supports the `platform` query parameter.

### Misc
- Updated Buildah to v1.22.3
- Updated the containers/storage library to v1.34.1
- Updated the containers/image library to v5.15.2
- Updated the containers/common library to v0.42.1

## 3.2.3
### Security
- This release addresses CVE-2021-3602, an issue with the `podman build` command with the `--isolation chroot` flag that results in environment variables from the host leaking into build containers.

### Bugfixes
- Fixed a bug where events related to images could occur before the relevant operation had completed (e.g. an image pull event could be written before the pull was finished) ([#10812](https://github.com/containers/podman/issues/10812)).
- Fixed a bug where `podman save` would refuse to save images with an architecture different from that of the host ([#10835](https://github.com/containers/podman/issues/10835)).
- Fixed a bug where the `podman import` command did not correctly handle images without tags ([#10854](https://github.com/containers/podman/issues/10854)).
- Fixed a bug where Podman's journald events backend would fail and prevent Podman from running when run on a host with systemd as PID1 but in an environment (e.g. a container) without systemd ([#10863](https://github.com/containers/podman/issues/10863)).
- Fixed a bug where containers using rootless CNI networking would fail to start when the `dnsname` CNI plugin was in use and the host system's `/etc/resolv.conf` was a symlink ([#10855](https://github.com/containers/podman/issues/10855) and [#10929](https://github.com/containers/podman/issues/10929)).
- Fixed a bug where containers using rootless CNI networking could fail to start due to a race in rootless CNI initialization ([#10930](https://github.com/containers/podman/issues/10930)).

### Misc
- Updated Buildah to v1.21.3
- Updated the containers/common library to v0.38.16

## 3.2.2
### Changes
- Podman's handling of the Architecture field of images has been relaxed. Since 3.2.0, Podman required that the architecture of the image match the architecture of the system to run containers based on an image, but images often incorrectly report architecture, causing Podman to reject valid images ([#10648](https://github.com/containers/podman/issues/10648) and [#10682](https://github.com/containers/podman/issues/10682)).
- Podman no longer uses inotify to monitor for changes to CNI configurations. This removes potential issues where Podman cannot be run because a user has exhausted their available inotify sessions ([#10686](https://github.com/containers/podman/issues/10686)).

### Bugfixes
- Fixed a bug where the `podman cp` would, when given a directory as its source and a target that existed and was a file, copy the contents of the directory into the parent directory of the file; this now results in an error.
- Fixed a bug where the `podman logs` command would, when following a running container's logs, not include the last line of output from the container when it exited when the `k8s-file` driver was in use ([#10675](https://github.com/containers/podman/issues/10675)).
- Fixed a bug where Podman would fail to run containers if `systemd-resolved` was incorrectly detected as the system's DNS server ([#10733](https://github.com/containers/podman/issues/10733)).
- Fixed a bug where the `podman exec -t` command would only resize the exec session's TTY after the session started, leading to a race condition where the terminal would initially not have a size set ([#10560](https://github.com/containers/podman/issues/10560)).
- Fixed a bug where Podman containers using the `slirp4netns` network mode would add an incorrect entry to `/etc/hosts` pointing the container's hostname to the wrong IP address.
- Fixed a bug where Podman would create volumes specified by images with incorrect permissions ([#10188](https://github.com/containers/podman/issues/10188) and [#10606](https://github.com/containers/podman/issues/10606)).
- Fixed a bug where Podman would not respect the `uid` and `gid` options to `podman volume create -o` ([#10620](https://github.com/containers/podman/issues/10620)).
- Fixed a bug where the `podman run` command could panic when parsing the system's cgroup configuration ([#10666](https://github.com/containers/podman/issues/10666)).
- Fixed a bug where the remote Podman client's `podman build -f - ...` command did not read a Containerfile from STDIN ([#10621](https://github.com/containers/podman/issues/10621)).
- Fixed a bug where the `podman container restore --import` command would fail to restore checkpoints created from privileged containers ([#10615](https://github.com/containers/podman/issues/10615)).
- Fixed a bug where Podman was not respecting the `TMPDIR` environment variable when pulling images ([#10698](https://github.com/containers/podman/issues/10698)).
- Fixed a bug where a number of Podman commands did not properly support using Go templates as an argument to the `--format` option.

### API
- Fixed a bug where the Compat Inspect endpoint for Containers did not include information on container healthchecks ([#10457](https://github.com/containers/podman/issues/10457)).
- Fixed a bug where the Libpod and Compat Build endpoints for Images did not properly handle the `devices` query parameter ([#10614](https://github.com/containers/podman/issues/10614)).

### Misc
- Fixed a bug where the Makefile's `make podman-remote-static` target to build a statically-linked `podman-remote` binary was instead producing dynamic binaries ([#10656](https://github.com/containers/podman/issues/10656)).
- Updated the containers/common library to v0.38.11

## 3.2.1
### Changes
- Podman now allows corrupt images (e.g. from restarting the system during an image pull) to be replaced by a `podman pull` of the same image (instead of requiring they be removed first, then re-pulled).

### Bugfixes
- Fixed a bug where Podman would fail to start containers if a Seccomp profile was not available at `/usr/share/containers/seccomp.json` ([#10556](https://github.com/containers/podman/issues/10556)).
- Fixed a bug where the `podman machine start` command failed on OS X machines with the AMD64 architecture and certain QEMU versions ([#10555](https://github.com/containers/podman/issues/10555)).
- Fixed a bug where Podman would always use the slow path for joining the rootless user namespace.
- Fixed a bug where the `podman stats` command would fail on Cgroups v1 systems when run on a container running systemd ([#10602](https://github.com/containers/podman/issues/10602)).
- Fixed a bug where pre-checkpoint support for `podman container checkpoint` did not function correctly.
- Fixed a bug where the remote Podman client's `podman build` command did not properly handle the `-f` option ([#9871](https://github.com/containers/podman/issues/9871)).
- Fixed a bug where the remote Podman client's `podman run` command would sometimes not resize the container's terminal before execution began ([#9859](https://github.com/containers/podman/issues/9859)).
- Fixed a bug where the `--filter` option to the `podman image prune` command was nonfunctional.
- Fixed a bug where the `podman logs -f` command would exit before all output for a container was printed when the `k8s-file` log driver was in use ([#10596](https://github.com/containers/podman/issues/10596)).
- Fixed a bug where Podman would not correctly detect that systemd-resolved was in use on the host and adjust DNS servers in the container appropriately under some circumstances ([#10570](https://github.com/containers/podman/issues/10570)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands acted improperly when containers were in the Created state, marking the changes as done but not actually performing them.

### API
- Fixed a bug where the Compat and Libpod Prune endpoints for Networks returned null, instead of an empty array, when nothing was pruned.
- Fixed a bug where the Create API for Images would continue to pull images even if a client closed the connection mid-pull ([#7558](https://github.com/containers/podman/issues/7558)).
- Fixed a bug where the Events API did not include some information (including labels) when sending events.
- Fixed a bug where the Events API would, when streaming was not requested, send at most one event ([#10529](https://github.com/containers/podman/issues/10529)).

### Misc
- Updated the containers/common library to v0.38.9

## 3.2.0
### Features
- Docker Compose is now supported with rootless Podman ([#9169](https://github.com/containers/podman/issues/9169)).
- The `podman network connect`, `podman network disconnect`, and `podman network reload` commands have been enabled for rootless Podman.
- An experimental new set of commands, `podman machine`, was added to assist in managing virtual machines containing a Podman server. These are intended for easing the use of Podman on OS X by handling the creation of a Linux VM for running Podman.
- The `podman generate kube` command can now be run on Podman named volumes (generating `PersistentVolumeClaim` YAML), in addition to pods and containers.
- The `podman play kube` command now supports two new options, `--ip` and `--mac`, to set static IPs and MAC addresses for created pods ([#8442](https://github.com/containers/podman/issues/8442) and [#9731](https://github.com/containers/podman/issues/9731)).
- The `podman play kube` command's support for `PersistentVolumeClaim` YAML has been greatly improved.
- The `podman generate kube` command now preserves the label used by `podman auto-update` to identify containers to update as a Kubernetes annotation, and the `podman play kube` command will convert this annotation back into a label. This allows `podman auto-update` to be used with containers created by `podman play kube`.
- The `podman play kube` command now supports Kubernetes `secretRef` YAML (using the secrets support from `podman secret`) for environment variables.
- Secrets can now be added to containers as environment variables using the `type=env` option to the `--secret` flag to `podman create` and `podman run`.
- The `podman start` command now supports the `--all` option, allowing all containers to be started simultaneously with a single command. The `--filter` option has also been added to filter which containers to start when `--all` is used.
- Filtering containers with the `--filter` option to `podman ps` and `podman start` now supports a new filter, `restart-policy`, to filter containers based on their restart policy.
- The `--group-add` option to rootless `podman run` and `podman create` now accepts a new value, `keep-groups`, which instructs Podman to retain the supplemental groups of the user running Podman in the created container. This is only supported with the `crun` OCI runtime.
- The `podman run` and `podman create` commands now support a new option, `--timeout`. This sets a maximum time the container is allowed to run, after which it is killed ([#6412](https://github.com/containers/podman/issues/6412)).
- The `podman run` and `podman create` commands now support a new option, `--pidfile`. This will create a file when the container is started containing the PID of the first process in the container.
- The `podman run` and `podman create` commands now support a new option, `--requires`. The `--requires` option adds dependency containers - containers that must be running before the current container. Commands like `podman start` will automatically start the requirements of a container before starting the container itself.
- Auto-updating containers can now be done with locally-built images, not just images hosted on a registry, by creating containers with the `io.containers.autoupdate` label set to `local`.
- Podman now supports the [Container Device Interface](https://github.com/container-orchestrated-devices/container-device-interface) (CDI) standard.
- Podman now adds an entry to `/etc/hosts`, `host.containers.internal`, pointing to the current gateway (which, for root containers, is usually a bridge interface on the host system) ([#5651](https://github.com/containers/podman/issues/5651)).
- The `podman ps`, `podman pod ps`, `podman network list`, `podman secret list`, and `podman volume list` commands now support a `--noheading` option, which will cause Podman to omit the heading line including column names.
- The `podman unshare` command now supports a new flag, `--rootless-cni`, to join the rootless network namespace. This allows commands to be run in the same network environment as rootless containers with CNI networking.
- The `--security-opt unmask=` option to `podman run` and `podman create` now supports glob operations to unmask a group of paths at once (e.g. `podman run --security-opt unmask=/proc/* ...` will unmask all paths in `/proc` in the container).
- The `podman network prune` command now supports a `--filter` option to filter which networks will be pruned.

### Changes
- The change in Podman 3.1.2 where the `:z` and `:Z` mount options for volumes were ignored for privileged containers has been reverted after discussion in [#10209](https://github.com/containers/podman/issues/10209).
- Podman's rootless CNI functionality no longer requires a sidecar container! The removal of the requirement for the `rootless-cni-infra` container means that rootless CNI is now usable on all architectures, not just AMD64, and no longer requires pulling an image ([#8709](https://github.com/containers/podman/issues/8709)).
- The Image handling code used by Podman has seen a major rewrite to improve code sharing with our other projects, Buildah and CRI-O. This should result in fewer bugs and performance gains in the long term. Work on this is still ongoing.
- The `podman auto-update` command now prunes previous versions of images after updating if they are unused, to prevent disk exhaustion after repeated updates ([#10190](https://github.com/containers/podman/issues/10190)).
- The `podman play kube` now treats environment variables configured as references to a `ConfigMap` as mandatory unless the `optional` parameter was set; this better matches the behavior of Kubernetes.
- Podman now supports the `--context=default` flag from Docker as a no-op for compatibility purposes.
- When Podman is run as root, but without `CAP_SYS_ADMIN` being available, it will run in a user namespace using the same code as rootless Podman (instead of failing outright).
- The `podman info` command now includes the path of the Seccomp profile Podman is using, available cgroup controllers, and whether Podman is connected to a remote service or running containers locally.
- Containers created with the `--rm` option now automatically use the `volatile` storage flag when available for their root filesystems, causing them not to write changes to disk as often as they will be removed at completion anyways. This should result in improved performance.
- The `podman generate systemd --new` command will now include environment variables referenced by the container in generated unit files if the value would be looked up from the system environment.
- Podman now requires that Conmon v2.0.24 be available.

### Bugfixes
- Fixed a bug where the remote Podman client's `podman build` command did not support the `--arch`, `--platform`, and `--os`, options.
- Fixed a bug where the remote Podman client's `podman build` command ignored the `--rm=false` option ([#9869](https://github.com/containers/podman/issues/9869)).
- Fixed a bug where the remote Podman client's `podman build --iidfile` command could include extra output (in addition to just the image ID) in the image ID file written ([#10233](https://github.com/containers/podman/issues/10233)).
- Fixed a bug where the remote Podman client's `podman build` command did not preserve hardlinks when moving files into the container via `COPY` instructions ([#9893](https://github.com/containers/podman/issues/9893)).
- Fixed a bug where the `podman generate systemd --new` command could generate extra `--iidfile` arguments if the container was already created with one.
- Fixed a bug where the `podman generate systemd --new` command would generate unit files that did not include `RequiresMountsFor` lines ([#10493](https://github.com/containers/podman/issues/10493)).
- Fixed a bug where the `podman generate kube` command produced incorrect YAML for containers which bind-mounted both `/` and `/root` from the host system into the container ([#9764](https://github.com/containers/podman/issues/9764)).
- Fixed a bug where pods created by `podman play kube` from YAML that specified `ShareProcessNamespace` would only share the PID namespace (and not also the UTS, Network, and IPC namespaces) ([#9128](https://github.com/containers/podman/issues/9128)).
- Fixed a bug where the `podman network reload` command could generate spurious error messages when `iptables-nft` was in use.
- Fixed a bug where rootless Podman could fail to attach to containers when the user running Podman had a large UID.
- Fixed a bug where the `podman ps` command could fail with a `no such container` error due to a race condition with container removal ([#10120](https://github.com/containers/podman/issues/10120)).
- Fixed a bug where containers using the `slirp4netns` network mode and setting a custom `slirp4netns` subnet while using the `rootlesskit` port forwarder would not be able to forward ports ([#9828](https://github.com/containers/podman/issues/9828)).
- Fixed a bug where the `--filter ancestor=` option to `podman ps` did not require an exact match of the image name/ID to include a container in its results.
- Fixed a bug where the `--filter until=` option to `podman image prune` would prune images created after the specified time (instead of before).
- Fixed a bug where setting a custom Seccomp profile via the `seccomp_profile` option in `containers.conf` had no effect, and the default profile was used instead.
- Fixed a bug where the `--cgroup-parent` option to `podman create` and `podman run` was ignored in rootless Podman on cgroups v2 systems with the `cgroupfs` cgroup manager ([#10173](https://github.com/containers/podman/issues/10173)).
- Fixed a bug where the `IMAGE` and `NAME` variables in `podman container runlabel` were not being correctly substituted ([#10192](https://github.com/containers/podman/issues/10192)).
- Fixed a bug where Podman could freeze when creating containers with a specific combination of volumes and working directory ([#10216](https://github.com/containers/podman/issues/10216)).
- Fixed a bug where rootless Podman containers restarted by restart policy (e.g. containers created with `--restart=always`) would lose networking after being restarted ([#8047](https://github.com/containers/podman/issues/8047)).
- Fixed a bug where the `podman cp` command could not copy files into containers created with the `--pid=host` flag ([#9985](https://github.com/containers/podman/issues/9985)).
- Fixed a bug where filters to the `podman events` command could not be specified twice (if a filter is specified more than once, it will match if any of the given values match - logical or) ([#10507](https://github.com/containers/podman/issues/10507)).
- Fixed a bug where Podman would include IPv6 nameservers in `resolv.conf` in containers without IPv6 connectivity ([#10158](https://github.com/containers/podman/issues/10158)).
- Fixed a bug where containers could not be created with static IP addresses when connecting to a network using the `macvlan` driver ([#10283](https://github.com/containers/podman/issues/10283)).

### API
- Fixed a bug where the Compat Create endpoint for Containers did not allow advanced network options to be set ([#10110](https://github.com/containers/podman/issues/10110)).
- Fixed a bug where the Compat Create endpoint for Containers ignored static IP information provided in the `IPAMConfig` block ([#10245](https://github.com/containers/podman/issues/10245)).
- Fixed a bug where the Compat Inspect endpoint for Containers returned null (instead of an empty list) for Networks when the container was not joined to a CNI network ([#9837](https://github.com/containers/podman/issues/9837)).
- Fixed a bug where the Compat Wait endpoint for Containers could miss containers exiting if they were immediately restarted.
- Fixed a bug where the Compat Create endpoint for Volumes required that the user provide a name for the new volume ([#9803](https://github.com/containers/podman/issues/9803)).
- Fixed a bug where the Libpod Info handler would sometimes not return the correct path to the Podman API socket.
- Fixed a bug where the Compat Events handler used the wrong name for container exited events (`died` instead of `die`) ([#10168](https://github.com/containers/podman/issues/10168)).
- Fixed a bug where the Compat Push endpoint for Images could leak goroutines if the remote end closed the connection prematurely.

### Misc
- Updated Buildah to v1.21.0
- Updated the containers/common library to v0.38.5
- Updated the containers/storage library to v1.31.3

## 3.1.2
### Bugfixes
- The Compat Export endpoint for Images now supports exporting multiple images at the same time to a single archive.
- Fixed a bug where images with empty layers were stored incorrectly, causing them to be unable to be pushed or saved.
- Fixed a bug where the `podman rmi` command could fail to remove corrupt images from storage.
- Fixed a bug where the remote Podman client's `podman save` command did not support the `oci-dir` and `docker-dir` formats ([#9742](https://github.com/containers/podman/issues/9742)).
- Fixed a bug where volume mounts from `podman play kube` created with a trailing `/` in the container path were were not properly superseding named volumes from the image ([#9618](https://github.com/containers/podman/issues/9618)).
- Fixed a bug where Podman could fail to build on 32-bit architectures.

### Misc
- Updated the containers/image library to v5.11.1

## 3.1.1
### Changes
- Podman now recognizes `trace` as a valid argument to the `--log-level` command. Trace logging is now the most verbose level of logging available.
- The `:z` and `:Z` options for volume mounts are now ignored when the container is privileged or is run with SELinux isolation disabled (`--security-opt label=disable`). This matches better matches Docker's behavior in this case.

### Bugfixes
- Fixed a bug where pruning images with the `podman image prune` or `podman system prune` commands could cause Podman to panic.
- Fixed a bug where the `podman save` command did not properly error when the `--compress` flag was used with incompatible format types.
- Fixed a bug where the `--security-opt` and `--ulimit` options to the remote Podman client's `podman build` command were nonfunctional.
- Fixed a bug where the `--log-rusage` option to the remote Podman client's `podman build` command was nonfunctional ([#9489](https://github.com/containers/podman/issues/9889)).
- Fixed a bug where the `podman build` command could, in some circumstances, use the wrong OCI runtime ([#9459](https://github.com/containers/podman/issues/9459)).
- Fixed a bug where the remote Podman client's `podman build` command could return 0 despite failing ([#10029](https://github.com/containers/podman/issues/10029)).
- Fixed a bug where the `podman container runlabel` command did not properly expand the `IMAGE` and `NAME` variables in the label ([#9405](https://github.com/containers/podman/issues/9405)).
- Fixed a bug where poststop OCI hooks would be executed twice on containers started with the `--rm` argument ([#9983](https://github.com/containers/podman/issues/9983)).
- Fixed a bug where rootless Podman could fail to launch containers on cgroups v2 systems when the `cgroupfs` cgroup manager was in use.
- Fixed a bug where the `podman stats` command could error when statistics tracked exceeded the maximum size of a 32-bit signed integer ([#9979](https://github.com/containers/podman/issues/9979)).
- Fixed a bug where rootless Podman containers run with `--userns=keepid` (without a `--user` flag in addition) would grant exec sessions run in them too many capabilities ([#9919](https://github.com/containers/podman/issues/9919)).
- Fixed a bug where the `--authfile` option to `podman build` did not validate that the path given existed ([#9572](https://github.com/containers/podman/issues/9572)).
- Fixed a bug where the `--storage-opt` option to Podman was appending to, instead of overriding (as is documented), the default storage options.
- Fixed a bug where the `podman system service` connection did not function properly when run in a socket-activated systemd unit file as a non-root user.
- Fixed a bug where the `--network` option to the `podman play kube` command of the remote Podman client was being ignored ([#9698](https://github.com/containers/podman/issues/9698)).
- Fixed a bug where the `--log-driver` option to the `podman play kube` command was nonfunctional ([#10015](https://github.com/containers/podman/issues/10015)).

### API
- Fixed a bug where the Libpod Create endpoint for Manifests did not properly validate the image the manifest was being created with.
- Fixed a bug where the Libpod DF endpoint could, in error cases, append an extra null to the JSON response, causing decode errors.
- Fixed a bug where the Libpod and Compat Top endpoint for Containers would return process names that included extra whitespace.
- Fixed a bug where the Compat Prune endpoint for Containers accepted too many types of filter.

### Misc
- Updated Buildah to v1.20.1
- Updated the containers/storage library to v1.29.0
- Updated the containers/image library to v5.11.0
- Updated the containers/common library to v0.36.0

## 3.1.0
### Features
- A set of new commands has been added to manage secrets! The `podman secret create`, `podman secret inspect`, `podman secret ls` and `podman secret rm` commands have been added to handle secrets, along with the `--secret` option to `podman run` and `podman create` to add secrets to containers. The initial driver for secrets does not support encryption - this will be added in a future release.
- A new command to prune networks, `podman network prune`, has been added ([#8673](https://github.com/containers/podman/issues/8673)).
- The `-v` option to `podman run` and `podman create` now supports a new volume option, `:U`, to chown the volume's source directory on the host to match the UID and GID of the container and prevent permissions issues ([#7778](https://github.com/containers/podman/issues/7778)).
- Three new commands, `podman network exists`, `podman volume exists`, and `podman manifest exists`, have been added to check for the existence of networks, volumes, and manifest lists.
- The `podman cp` command can now copy files into directories mounted as `tmpfs` in a running container.
- The `podman volume prune` command will now list volumes that will be pruned when prompting the user whether to continue and perform the prune ([#8913](https://github.com/containers/podman/issues/8913)).
- The Podman remote client's `podman build` command now supports the `--disable-compression`, `--excludes`, and `--jobs` options.
- The Podman remote client's `podman push` command now supports the `--format` option.
- The Podman remote client's `podman rm` command now supports the `--all` and `--ignore` options.
- The Podman remote client's `podman search` command now supports the `--no-trunc` and `--list-tags` options.
- The `podman play kube` command can now read in Kubernetes YAML from `STDIN` when `-` is specified as file name (`podman play kube -`), allowing input to be piped into the command for scripting ([#8996](https://github.com/containers/podman/issues/8996)).
- The `podman generate systemd` command now supports a `--no-header` option, which disables creation of the header comment automatically added by Podman to generated unit files.
- The `podman generate kube` command can now generate `PersistentVolumeClaim` YAML for Podman named volumes ([#5788](https://github.com/containers/podman/issues/5788)).
- The `podman generate kube` command can now generate YAML files containing multiple resources (pods or deployments) ([#9129](https://github.com/containers/podman/issues/9129)).

### Security
- This release resolves CVE-2021-20291, a deadlock vulnerability in the storage library caused by pulling a specially-crafted container image.

### Changes
- The Podman remote client's `podman build` command no longer allows the `-v` flag to be used. Volumes are not yet supported with remote Podman when the client and service are on different machines.
- The `podman kill` and `podman stop` commands now print the name given by the user for each container, instead of the full ID.
- When the `--security-opt unmask=ALL` or `--security-opt unmask=/sys/fs/cgroup` options to `podman create` or `podman run` are given, Podman will mount cgroups into the container as read-write, instead of read-only ([#8441](https://github.com/containers/podman/issues/8441)).
- The `podman rmi` command has been changed to better handle cases where an image is incomplete or corrupted, which can be caused by interrupted image pulls.
- The `podman rename` command has been improved to be more atomic, eliminating many race conditions that could potentially render a renamed container unusable.
- Detection of which OCI runtimes run using virtual machines and thus require custom SELinux labelling has been improved ([#9582](https://github.com/containers/podman/issues/9582)).
- The hidden `--trace` option to `podman` has been turned into a no-op. It was used in very early versions for performance tracing, but has not been supported for some time.
- The `podman generate systemd` command now generates `RequiresMountsFor` lines to ensure necessary storage directories are mounted before systemd starts Podman.
- Podman will now emit a warning when `--tty` and `--interactive` are both passed, but `STDIN` is not a TTY. This will be made into an error in the next major Podman release some time next year.

### Bugfixes
- Fixed a bug where rootless Podman containers joined to CNI networks could not receive traffic from forwarded ports ([#9065](https://github.com/containers/podman/issues/9065)).
- Fixed a bug where `podman network create` with the `--macvlan` flag did not honor the `--gateway`, `--subnet`, and `--opt` options ([#9167](https://github.com/containers/podman/issues/9167)).
- Fixed a bug where the `podman generate kube` command generated invalid YAML for privileged containers ([#8897](https://github.com/containers/podman/issues/8897)).
- Fixed a bug where the `podman generate kube` command could not be used with containers that were not running.
- Fixed a bug where the `podman generate systemd` command could duplicate some parameters to Podman in generated unit files ([#9776](https://github.com/containers/podman/issues/9776)).
- Fixed a bug where Podman did not add annotations specified in `containers.conf` to containers.
- Foxed a bug where Podman did not respect the `no_hosts` default in `containers.conf` when creating containers.
- Fixed a bug where the `--tail=0`, `--since`, and `--follow` options to the `podman logs` command did not function properly when using the `journald` log backend.
- Fixed a bug where specifying more than one container to `podman logs` when the `journald` log backend was in use did not function correctly.
- Fixed a bug where the `podman run` and `podman create` commands would panic if a memory limit was set, but the swap limit was set to unlimited ([#9429](https://github.com/containers/podman/issues/9429)).
- Fixed a bug where the `--network` option to `podman run`, `podman create`, and `podman pod create` would error if the user attempted to specify CNI networks by ID, instead of name ([#9451](https://github.com/containers/podman/issues/9451)).
- Fixed a bug where Podman's cgroup handling for cgroups v1 systems did not properly handle cases where a cgroup existed on some, but not all, controllers, resulting in errors from the `podman stats` command ([#9252](https://github.com/containers/podman/issues/9252)).
- Fixed a bug where the `podman cp` did not properly handle cases where `/dev/stdout` was specified as the destination (it was treated identically to `-`) ([#9362](https://github.com/containers/podman/issues/9362)).
- Fixed a bug where the `podman cp` command would create files with incorrect ownership ([#9526](https://github.com/containers/podman/issues/9626)).
- Fixed a bug where the `podman cp` command did not properly handle cases where the destination directory did not exist.
- Fixed a bug where the `podman cp` command did not properly evaluate symlinks when copying out of containers.
- Fixed a bug where the `podman rm -fa` command would error when attempting to remove containers created with `--rm` ([#9479](https://github.com/containers/podman/issues/9479)).
- Fixed a bug where the ordering of capabilities was nondeterministic in the `CapDrop` field of the output of `podman inspect` on a container ([#9490](https://github.com/containers/podman/issues/9490)).
- Fixed a bug where the `podman network connect` command could be used with containers that were not initially connected to a CNI bridge network (e.g. containers created with `--net=host`) ([#9496](https://github.com/containers/podman/issues/9496)).
- Fixed a bug where DNS search domains required by the `dnsname` CNI plugin were not being added to container's `resolv.conf` under some circumstances.
- Fixed a bug where the `--ignorefile` option to `podman build` was nonfunctional ([#9570](https://github.com/containers/podman/issues/9570)).
- Fixed a bug where the `--timestamp` option to `podman build` was nonfunctional ([#9569](https://github.com/containers/podman/issues/9569)).
- Fixed a bug where the `--iidfile` option to `podman build` could cause Podman to panic if an error occurred during the build.
- Fixed a bug where the `--dns-search` option to `podman build` was nonfunctional ([#9574](https://github.com/containers/podman/issues/9574)).
- Fixed a bug where the `--pull-never` option to `podman build` was nonfunctional ([#9573](https://github.com/containers/podman/issues/9573)).
- Fixed a bug where the `--build-arg` option to `podman build` would, when given a key but not a value, error (instead of attempting to look up the key as an environment variable) ([#9571](https://github.com/containers/podman/issues/9571)).
- Fixed a bug where the `--isolation` option to `podman build` in the remote Podman client was nonfunctional.
- Fixed a bug where the `podman network disconnect` command could cause errors when the container that had a network removed was stopped and its network was cleaned up ([#9602](https://github.com/containers/podman/issues/9602)).
- Fixed a bug where the `podman network rm` command did not properly check what networks a container was present in, resulting in unexpected behavior if `podman network connect` or `podman network disconnect` had been used with the network ([#9632](https://github.com/containers/podman/issues/9632)).
- Fixed a bug where some errors with stopping a container could cause Podman to panic, and the container to be stuck in an unusable `stopping` state ([#9615](https://github.com/containers/podman/issues/9615)).
- Fixed a bug where the `podman load` command could return 0 even in cases where an error occurred ([#9672](https://github.com/containers/podman/issues/9672)).
- Fixed a bug where specifying storage options to Podman using the `--storage-opt` option would override all storage options. Instead, storage options are now overridden only when the `--storage-driver` option is used to override the current graph driver ([#9657](https://github.com/containers/podman/issues/9657)).
- Fixed a bug where containers created with `--privileged` could request more capabilities than were available to Podman.
- Fixed a bug where `podman commit` did not use the `TMPDIR` environment variable to place temporary files created during the commit ([#9825](https://github.com/containers/podman/issues/9825)).
- Fixed a bug where remote Podman could error when attempting to resize short-lived containers ([#9831](https://github.com/containers/podman/issues/9831)).
- Fixed a bug where Podman was unusable on kernels built without `CONFIG_USER_NS`.
- Fixed a bug where the ownership of volumes created by `podman volume create` and then mounted into a container could be incorrect ([#9608](https://github.com/containers/podman/issues/9608)).
- Fixed a bug where Podman volumes using a volume plugin could not pass certain options, and could not be used as non-root users.
- Fixed a bug where the `--tz` option to `podman create` and `podman run` did not properly validate its input.

### API
- Fixed a bug where the `X-Registry-Auth` header did not accept `null` as a valid value.
- A new compat endpoint, `/auth`, has been added. This endpoint validates credentials against a registry ([#9564](https://github.com/containers/podman/issues/9564)).
- Fixed a bug where the compat Build endpoint for Images specified labels using the wrong type (array vs map). Both formats will be accepted now.
- Fixed a bug where the compat Build endpoint for Images did not report that it successfully tagged the built image in its response.
- Fixed a bug where the compat Create endpoint for Images did not provide progress information on pulling the image in its response.
- Fixed a bug where the compat Push endpoint for Images did not properly handle the destination (used a query parameter, instead of a path parameter).
- Fixed a bug where the compat Push endpoint for Images did not send the progress of the push and the digest of the pushed image in the response body.
- Fixed a bug where the compat List endpoint for Networks returned null, instead of an empty array (`[]`), when no networks were present ([#9293](https://github.com/containers/podman/issues/9293)).
- Fixed a bug where the compat List endpoint for Networks returned nulls, instead of empty maps, for networks that do not have Labels and/or Options.
- The Libpod Inspect endpoint for networks (`/libpod/network/$ID/json`) now has an alias at `/libpod/network/$ID` ([#9691](https://github.com/containers/podman/issues/9691)).
- Fixed a bug where the libpod Inspect endpoint for Networks returned a 1-size array of results, instead of a single result ([#9690](https://github.com/containers/podman/issues/9690)).
- The Compat List endpoint for Networks now supports the legacy format for filters in parallel with the current filter format ([#9526](https://github.com/containers/podman/issues/9526)).
- Fixed a bug where the compat Create endpoint for Containers did not properly handle tmpfs filesystems specified with options ([#9511](https://github.com/containers/podman/issues/9511)).
- Fixed a bug where the compat Create endpoint for Containers did not create bind-mount source directories ([#9510](https://github.com/containers/podman/issues/9510)).
- Fixed a bug where the compat Create endpoint for Containers did not properly handle the `NanoCpus` option ([#9523](https://github.com/containers/podman/issues/9523)).
- Fixed a bug where the Libpod create endpoint for Containers has a misnamed field in its JSON.
- Fixed a bug where the compat List endpoint for Containers did not populate information on forwarded ports ([#9553](https://github.com/containers/podman/issues/9553))
- Fixed a bug where the compat List endpoint for Containers did not populate information on container CNI networks ([#9529](https://github.com/containers/podman/issues/9529)).
- Fixed a bug where the compat and libpod Stop endpoints for Containers would ignore a timeout of 0.
- Fixed a bug where the compat and libpod Resize endpoints for Containers did not set the correct terminal sizes (dimensions were reversed) ([#9756](https://github.com/containers/podman/issues/9756)).
- Fixed a bug where the compat Remove endpoint for Containers would not return 404 when attempting to remove a container that does not exist ([#9675](https://github.com/containers/podman/issues/9675)).
- Fixed a bug where the compat Prune endpoint for Volumes would still prune even if an invalid filter was specified.
- Numerous bugs related to filters have been addressed.

### Misc
- Updated Buildah to v1.20.0
- Updated the containers/storage library to v1.28.1
- Updated the containers/image library to v5.10.5
- Updated the containers/common library to v0.35.4

## 3.0.1
### Changes
- Several frequently-occurring `WARN` level log messages have been downgraded to `INFO` or `DEBUG` to not clutter terminal output.

### Bugfixes
- Fixed a bug where the `Created` field of `podman ps --format=json` was formatted as a string instead of an Unix timestamp (integer) ([#9315](https://github.com/containers/podman/issues/9315)).
- Fixed a bug where failing lookups of individual layers during the `podman images` command would cause the whole command to fail without printing output.
- Fixed a bug where `--cgroups=split` did not function properly on cgroups v1 systems.
- Fixed a bug where mounting a volume over an directory in the container that existed, but was empty, could fail ([#9393](https://github.com/containers/podman/issues/9393)).
- Fixed a bug where mounting a volume over a directory in the container that existed could copy the entirety of the container's rootfs, instead of just the directory mounted over, into the volume ([#9415](https://github.com/containers/podman/pull/9415)).
- Fixed a bug where Podman would treat the `--entrypoint=[""]` option to `podman run` and `podman create` as a literal empty string in the entrypoint, when instead it should have been ignored ([#9377](https://github.com/containers/podman/issues/9377)).
- Fixed a bug where Podman would set the `HOME` environment variable to `""` when the container ran as a user without an assigned home directory ([#9378](https://github.com/containers/podman/issues/9378)).
- Fixed a bug where specifying a pod infra image that had no tags (by using its ID) would cause `podman pod create` to panic ([#9374](https://github.com/containers/podman/issues/9374)).
- Fixed a bug where the `--runtime` option was not properly handled by the `podman build` command ([#9365](https://github.com/containers/podman/issues/9365)).
- Fixed a bug where Podman would incorrectly print an error message related to the remote API when the remote API was not in use and starting Podman failed.
- Fixed a bug where Podman would change ownership of a container's working directory, even if it already existed ([#9387](https://github.com/containers/podman/issues/9387)).
- Fixed a bug where the `podman generate systemd --new` command would incorrectly escape `%t` when generating the path for the PID file ([#9373](https://github.com/containers/podman/issues/9373)).
- Fixed a bug where Podman could, when run inside a Podman container with the host's containers/storage directory mounted into the container, erroneously detect a reboot and reset container state if the temporary directory was not also mounted in ([#9191](https://github.com/containers/podman/issues/9191)).
- Fixed a bug where some options of the `podman build` command (including but not limited to `--jobs`) were nonfunctional ([#9247](https://github.com/containers/podman/issues/9247)).

### API
- Fixed a breaking change to the Libpod Wait API for Containers where the Conditions parameter changed type in Podman v3.0 ([#9351](https://github.com/containers/podman/issues/9351)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle forwarded ports that did not specify a host port.
- Fixed a bug where the Libpod Wait endpoint for Containers could write duplicate headers after an error occurred.
- Fixed a bug where the Compat Create endpoint for Images would not pull images that already had a matching tag present locally, even if a more recent version was available at the registry ([#9232](https://github.com/containers/podman/issues/9232)).
- The Compat Create endpoint for Images has had its compatibility with Docker improved, allowing its use with the `docker-java` library.

### Misc
- Updated Buildah to v1.19.4
- Updated the containers/storage library to v1.24.6

## 3.0.0
### Features
- Podman now features initial support for Docker Compose.
- Added the `podman rename` command, which allows containers to be renamed after they are created ([#1925](https://github.com/containers/podman/issues/1925)).
- The Podman remote client now supports the `podman copy` command.
- A new command, `podman network reload`, has been added. This command will re-configure the network of all running containers, and can be used to recreate firewall rules lost when the system firewall was reloaded (e.g. via `firewall-cmd --reload`).
- Podman networks now have IDs. They can be seen in `podman network ls` and can be used when removing and inspecting networks. Existing networks receive IDs automatically.
- Podman networks now also support labels. They can be added via the `--label` option to `network create`, and `podman network ls` can filter labels based on them.
- The `podman network create` command now supports setting bridge MTU and VLAN through the `--opt` option ([#8454](https://github.com/containers/podman/issues/8454)).
- The `podman container checkpoint` and `podman container restore` commands can now checkpoint and restore containers that include volumes.
- The `podman container checkpoint` command now supports the `--with-previous` and `--pre-checkpoint` options, and the `podman container restore` command now support the `--import-previous` option. These add support for two-step checkpointing with lowered dump times.
- The `podman push` command can now push manifest lists. Podman will first attempt to push as an image, then fall back to pushing as a manifest list if that fails.
- The `podman generate kube` command can now be run on multiple containers at once, and will generate a single pod containing all of them.
- The `podman generate kube` and `podman play kube` commands now support Kubernetes DNS configuration, and will preserve custom DNS configuration when exporting or importing YAML ([#9132](https://github.com/containers/podman/issues/9132)).
- The `podman generate kube` command now properly supports generating YAML for containers and pods creating using host networking (`--net=host`) ([#9077](https://github.com/containers/podman/issues/9077)).
- The `podman kill` command now supports a `--cidfile` option to kill containers given a file containing the container's ID ([#8443](https://github.com/containers/podman/issues/8443)).
- The `podman pod create` command now supports the `--net=none` option ([#9165](https://github.com/containers/podman/issues/9165)).
- The `podman volume create` command can now specify volume UID and GID as options with the `UID` and `GID` fields passed to the the `--opt` option.
- Initial support has been added for Docker Volume Plugins. Podman can now define available plugins in `containers.conf` and use them to create volumes with `podman volume create --driver`.
- The `podman run` and `podman create` commands now support a new option, `--platform`, to specify the platform of the image to be used when creating the container.
- The `--security-opt` option to `podman run` and `podman create` now supports the `systempaths=unconfined` option to unrestrict access to all paths in the container, as well as `mask` and `unmask` options to allow more granular restriction of container paths.
- The `podman stats --format` command now supports a new format specified, `MemUsageBytes`, which prints the raw bytes of memory consumed by a container without human-readable formatting [#8945](https://github.com/containers/podman/issues/8945).
- The `podman ps` command can now filter containers based on what pod they are joined to via the `pod` filter ([#8512](https://github.com/containers/podman/issues/8512)).
- The `podman pod ps` command can now filter pods based on what networks they are joined to via the `network` filter.
- The `podman pod ps` command can now print information on what networks a pod is joined to via the `.Networks` specifier to the `--format` option.
- The `podman system prune` command now supports filtering what containers, pods, images, and volumes will be pruned.
- The `podman volume prune` commands now supports filtering what volumes will be pruned.
- The `podman system prune` command now includes information on space reclaimed ([#8658](https://github.com/containers/podman/issues/8658)).
- The `podman info` command will now properly print information about packages in use on Gentoo and Arch systems.
- The `containers.conf` file now contains an option for disabling creation of a new kernel keyring on container creation ([#8384](https://github.com/containers/podman/issues/8384)).
- The `podman image sign` command can now sign multi-arch images by producing a signature for each image in a given manifest list.
- The `podman image sign` command, when run as rootless, now supports per-user registry configuration files in `$HOME/.config/containers/registries.d`.
- Configuration options for `slirp4netns` can now be set system-wide via the `NetworkCmdOptions` configuration option in `containers.conf`.
- The MTU of `slirp4netns` can now be configured via the `mtu=` network command option (e.g. `podman run --net slirp4netns:mtu=9000`).

### Security
- A fix for CVE-2021-20199 is included. Podman between v1.8.0 and v2.2.1 used `127.0.0.1` as the source address for all traffic forwarded into rootless containers by a forwarded port; this has been changed to address the issue.

### Changes
- Shortname aliasing support has now been turned on by default. All Podman commands that must pull an image will, if a TTY is available, prompt the user about what image to pull.
- The `podman load` command no longer accepts a `NAME[:TAG]` argument. The presence of this argument broke CLI compatibility with Docker by making `docker load` commands unusable with Podman ([#7387](https://github.com/containers/podman/issues/7387)).
- The Go bindings for the HTTP API have been rewritten with a focus on limiting dependency footprint and improving extensibility. Read more [here](https://github.com/containers/podman/blob/v3.0/pkg/bindings/README.md).
- The legacy Varlink API has been completely removed from Podman.
- The default log level for Podman has been changed from Error to Warn.
- The `podman network create` command can now create `macvlan` networks using the `--driver macvlan` option for Docker compatibility. The existing `--macvlan` flag has been deprecated and will be removed in Podman 4.0 some time next year.
- The `podman inspect` command has had the `LogPath` and `LogTag` fields moved into the `LogConfig` structure (from the root of the Inspect structure). The maximum size of the log file is also included.
- The `podman generate systemd` command no longer generates unit files using the deprecated `KillMode=none` option ([#8615](https://github.com/containers/podman/issues/8615)).
- The `podman stop` command now releases the container lock while waiting for it to stop - as such, commands like `podman ps` will no longer block until `podman stop` completes ([#8501](https://github.com/containers/podman/issues/8501)).
- Networks created with `podman network create --internal` no longer use the `dnsname` plugin. This configuration never functioned as expected.
- Error messages for the remote Podman client have been improved when it cannot connect to a Podman service.
- Error messages for `podman run` when an invalid SELinux is specified have been improved.
- Rootless Podman features improved support for containers with a single user mapped into the rootless user namespace.
- Pod infra containers now respect default sysctls specified in `containers.conf` allowing for advanced configuration of the namespaces they will share.
- SSH public key handling for remote Podman has been improved.

### Bugfixes
- Fixed a bug where the `podman history --no-trunc` command would truncate the `Created By` field ([#9120](https://github.com/containers/podman/issues/9120)).
- Fixed a bug where root containers that did not explicitly specify a CNI network to join did not generate an entry for the network in use in the `Networks` field of the output of `podman inspect` ([#6618](https://github.com/containers/podman/issues/6618)).
- Fixed a bug where, under some circumstances, container working directories specified by the image (via the `WORKDIR` instruction) but not present in the image, would not be created ([#9040](https://github.com/containers/podman/issues/9040)).
- Fixed a bug where the `podman generate systemd` command would generate invalid unit files if the container was creating using a command line that included doubled braces (`{{` and `}}`), e.g. `--log-opt-tag={{.Name}}` ([#9034](https://github.com/containers/podman/issues/9034)).
- Fixed a bug where the `podman generate systemd --new` command could generate unit files including invalid Podman commands if the container was created using merged short options (e.g. `podman run -dt`) ([#8847](https://github.com/containers/podman/issues/8847)).
- Fixed a bug where the `podman generate systemd --new` command could generate unit files that did not handle Podman commands including some special characters (e.g. `$`) ([#9176](https://github.com/containers/podman/issues/9176)
- Fixed a bug where rootless containers joining CNI networks could not set a static IP address ([#7842](https://github.com/containers/podman/issues/7842)).
- Fixed a bug where rootless containers joining CNI networks could not set network aliases ([#8567](https://github.com/containers/podman/issues/8567)).
- Fixed a bug where the remote client could, under some circumstances, not include the `Containerfile` when sending build context to the server ([#8374](https://github.com/containers/podman/issues/8374)).
- Fixed a bug where rootless Podman did not mount `/sys` as a new `sysfs` in some circumstances where it was acceptable.
- Fixed a bug where rootless containers that both joined a user namespace and a CNI networks would cause a segfault. These options are incompatible and now return an error.
- Fixed a bug where the `podman play kube` command did not properly handle `CMD` and `ARGS` from images ([#8803](https://github.com/containers/podman/issues/8803)).
- Fixed a bug where the `podman play kube` command did not properly handle environment variables from images ([#8608](https://github.com/containers/podman/issues/8608)).
- Fixed a bug where the `podman play kube` command did not properly print errors that occurred when starting containers.
- Fixed a bug where the `podman play kube` command errored when `hostNetwork` was used ([#8790](https://github.com/containers/podman/issues/8790)).
- Fixed a bug where the `podman play kube` command would always pull images when the `:latest` tag was specified, even if the image was available locally ([#7838](https://github.com/containers/podman/issues/7838)).
- Fixed a bug where the `podman play kube` command did not properly handle SELinux configuration, rending YAML with custom SELinux configuration unusable ([#8710](https://github.com/containers/podman/issues/8710)).
- Fixed a bug where the `podman generate kube` command incorrectly populated the `args` and `command` fields of generated YAML ([#9211](https://github.com/containers/podman/issues/9211)).
- Fixed a bug where containers in a pod would create a duplicate entry in the pod's shared `/etc/hosts` file every time the container restarted ([#8921](https://github.com/containers/podman/issues/8921)).
- Fixed a bug where the `podman search --list-tags` command did not support the `--format` option ([#8740](https://github.com/containers/podman/issues/8740)).
- Fixed a bug where the `http_proxy` option in `containers.conf` was not being respected, and instead was set unconditionally to true ([#8843](https://github.com/containers/podman/issues/8843)).
- Fixed a bug where rootless Podman could, on systems with a recent Conmon and users with a long username, fail to attach to containers ([#8798](https://github.com/containers/podman/issues/8798)).
- Fixed a bug where the `podman images` command would break and fail to display any images if an empty manifest list was present in storage ([#8931](https://github.com/containers/podman/issues/8931)).
- Fixed a bug where locale environment variables were not properly passed on to Conmon.
- Fixed a bug where Podman would not build on the MIPS architecture ([#8782](https://github.com/containers/podman/issues/8782)).
- Fixed a bug where rootless Podman could fail to properly configure user namespaces for rootless containers when the user specified a `--uidmap` option that included a mapping beginning with UID `0`.
- Fixed a bug where the `podman logs` command using the `k8s-file` backend did not properly handle partial log lines with a length of 1 ([#8879](https://github.com/containers/podman/issues/8879)).
- Fixed a bug where the `podman logs` command with the `--follow` option did not properly handle log rotation ([#8733](https://github.com/containers/podman/issues/8733)).
- Fixed a bug where user-specified `HOSTNAME` environment variables were overwritten by Podman ([#8886](https://github.com/containers/podman/issues/8886)).
- Fixed a bug where Podman would applied default sysctls from `containers.conf` in too many situations (e.g. applying network sysctls when the container shared its network with a pod).
- Fixed a bug where Podman did not properly handle cases where a secondary image store was in use and an image was present in both the secondary and primary stores ([#8176](https://github.com/containers/podman/issues/8176)).
- Fixed a bug where systemd-managed rootless Podman containers where the user in the container was not root could fail as the container's PID file was not accessible to systemd on the host ([#8506](https://github.com/containers/podman/issues/8506)).
- Fixed a bug where the `--privileged` option to `podman run` and `podman create` would, under some circumstances, not disable Seccomp ([#8849](https://github.com/containers/podman/issues/8849)).
- Fixed a bug where the `podman exec` command did not properly add capabilities when the container or exec session were run with `--privileged`.
- Fixed a bug where rootless Podman would use the `--enable-sandbox` option to `slirp4netns` unconditionally, even when `pivot_root` was disabled, rendering `slirp4netns` unusable when `pivot_root` was disabled ([#8846](https://github.com/containers/podman/issues/8846)).
- Fixed a bug where `podman build --logfile` did not actually write the build's log to the logfile.
- Fixed a bug where the `podman system service` command did not close STDIN, and could display user-interactive prompts ([#8700](https://github.com/containers/podman/issues/8700)).
- Fixed a bug where the `podman system reset` command could, under some circumstances, remove all the contents of the `XDG_RUNTIME_DIR` directory ([#8680](https://github.com/containers/podman/issues/8680)).
- Fixed a bug where the `podman network create` command created CNI configurations that did not include a default gateway ([#8748](https://github.com/containers/podman/issues/8748)).
- Fixed a bug where the `podman.service` systemd unit provided by default used the wrong service type, and would cause systemd to not correctly register the service as started ([#8751](https://github.com/containers/podman/issues/8751)).
- Fixed a bug where, if the `TMPDIR` environment variable was set for the container engine in `containers.conf`, it was being ignored.
- Fixed a bug where the `podman events` command did not properly handle future times given to the `--until` option ([#8694](https://github.com/containers/podman/issues/8694)).
- Fixed a bug where the `podman logs` command wrote container `STDERR` logs to `STDOUT` instead of `STDERR` ([#8683](https://github.com/containers/podman/issues/8683)).
- Fixed a bug where containers created from an image with multiple tags would report that they were created from the wrong tag ([#8547](https://github.com/containers/podman/issues/8547)).
- Fixed a bug where container capabilities were not set properly when the `--cap-add=all` and `--user` options to `podman create` and `podman run` were combined.
- Fixed a bug where the `--layers` option to `podman build` was nonfunctional ([#8643](https://github.com/containers/podman/issues/8643)).
- Fixed a bug where the `podman system prune` command did not act recursively, and thus would leave images, containers, pods, and volumes present that would be removed by a subsequent call to `podman system prune` ([#7990](https://github.com/containers/podman/issues/7990)).
- Fixed a bug where the `--publish` option to `podman run` and `podman create` did not properly handle ports specified as a range of ports with no host port specified ([#8650](https://github.com/containers/podman/issues/8650)).
- Fixed a bug where `--format` did not support JSON output for individual fields ([#8444](https://github.com/containers/podman/issues/8444)).
- Fixed a bug where the `podman stats` command would fail when run on root containers using the `slirp4netns` network mode ([#7883](https://github.com/containers/podman/issues/7883)).
- Fixed a bug where the Podman remote client would ask for a password even if the server's SSH daemon did not support password authentication ([#8498](https://github.com/containers/podman/issues/8498)).
- Fixed a bug where the `podman stats` command would fail if the system did not support one or more of the cgroup controllers Podman supports ([#8588](https://github.com/containers/podman/issues/8588)).
- Fixed a bug where the `--mount` option to `podman create` and `podman run` did not ignore the `consistency` mount option.
- Fixed a bug where failures during the resizing of a container's TTY would print the wrong error.
- Fixed a bug where the `podman network disconnect` command could cause the `podman inspect` command to fail for a container until it was restarted ([#9234](https://github.com/containers/podman/issues/9234)).
- Fixed a bug where containers created from a read-only rootfs (using the `--rootfs` option to `podman create` and `podman run`) would fail ([#9230](https://github.com/containers/podman/issues/9230)).
- Fixed a bug where specifying Go templates to the `--format` option to multiple Podman commands did not support the `join` function ([#8773](https://github.com/containers/podman/issues/8773)).
- Fixed a bug where the `podman rmi` command could, when run in parallel on multiple images, return `layer not known` errors ([#6510](https://github.com/containers/podman/issues/6510)).
- Fixed a bug where the `podman inspect` command on containers displayed unlimited ulimits incorrectly ([#9303](https://github.com/containers/podman/issues/9303)).
- Fixed a bug where Podman would fail to start when a volume was mounted over a directory in a container that contained symlinks that terminated outside the directory and its subdirectories ([#6003](https://github.com/containers/podman/issues/6003)).

### API
- Libpod API version has been bumped to v3.0.0.
- All Libpod Pod APIs have been modified to properly report errors with individual containers. Cases where the operation as a whole succeeded but individual containers failed now report an HTTP 409 error ([#8865](https://github.com/containers/podman/issues/8865)).
- The Compat API for Containers now supports the Rename and Copy APIs.
- Fixed a bug where the Compat Prune APIs (for volumes, containers, and images) did not return the amount of space reclaimed in their responses.
- Fixed a bug where the Compat and Libpod Exec APIs for Containers would drop errors that occurred prior to the exec session successfully starting (e.g. a "no such file" error if an invalid executable was passed) ([#8281](https://github.com/containers/podman/issues/8281))
- Fixed a bug where the Volumes field in the Compat Create API for Containers was being ignored ([#8649](https://github.com/containers/podman/issues/8649)).
- Fixed a bug where the NetworkMode field in the Compat Create API for Containers was not handling some values, e.g. `container:`, correctly.
- Fixed a bug where the Compat Create API for Containers did not set container name properly.
- Fixed a bug where containers created using the Compat Create API unconditionally used Kubernetes file logging (the default specified in `containers.conf` is now used).
- Fixed a bug where the Compat Inspect API for Containers could include container states not recognized by Docker.
- Fixed a bug where Podman did not properly clean up after calls to the Events API when the `journald` backend was in use, resulting in a leak of file descriptors ([#8864](https://github.com/containers/podman/issues/8864)).
- Fixed a bug where the Libpod Pull endpoint for Images could fail with an `index out of range` error under certain circumstances ([#8870](https://github.com/containers/podman/issues/8870)).
- Fixed a bug where the Libpod Exists endpoint for Images could panic.
- Fixed a bug where the Compat List API for Containers did not support all filters ([#8860](https://github.com/containers/podman/issues/8860)).
- Fixed a bug where the Compat List API for Containers did not properly populate the Status field.
- Fixed a bug where the Compat and Libpod Resize APIs for Containers ignored the height and width parameters ([#7102](https://github.com/containers/podman/issues/7102)).
- Fixed a bug where the Compat Search API for Images returned an incorrectly-formatted JSON response ([#8758](https://github.com/containers/podman/pull/8758)).
- Fixed a bug where the Compat Load API for Images did not properly clean up temporary files.
- Fixed a bug where the Compat Create API for Networks could panic when an empty IPAM configuration was specified.
- Fixed a bug where the Compat Inspect and List APIs for Networks did not include Scope.
- Fixed a bug where the Compat Wait endpoint for Containers did not support the same wait conditions that Docker did.

### Misc
- Updated Buildah to v1.19.2
- Updated the containers/storage library to v1.24.5
- Updated the containers/image library to v5.10.2
- Updated the containers/common library to v0.33.4

## v2.2.1
### Changes
- Due to a conflict with a previously-removed field, we were forced to modify the way image volumes (mounting images into containers using `--mount type=image`) were handled in the database. As a result, containers created in Podman 2.2.0 with image volumes will not have them in v2.2.1, and these containers will need to be re-created.

### Bugfixes
- Fixed a bug where rootless Podman would, on systems without the `XDG_RUNTIME_DIR` environment variable defined, use an incorrect path for the PID file of the Podman pause process, causing Podman to fail to start ([#8539](https://github.com/containers/podman/issues/8539)).
- Fixed a bug where containers created using Podman v1.7 and earlier were unusable in Podman due to JSON decode errors ([#8613](https://github.com/containers/podman/issues/8613)).
- Fixed a bug where Podman could retrieve invalid cgroup paths, instead of erroring, for containers that were not running.
- Fixed a bug where the `podman system reset` command would print a warning about a duplicate shutdown handler being registered.
- Fixed a bug where rootless Podman would attempt to mount `sysfs` in circumstances where it was not allowed; some OCI runtimes (notably `crun`) would fall back to alternatives and not fail, but others (notably `runc`) would fail to run containers.
- Fixed a bug where the `podman run` and `podman create` commands would fail to create containers from untagged images ([#8558](https://github.com/containers/podman/issues/8558)).
- Fixed a bug where remote Podman would prompt for a password even when the server did not support password authentication ([#8498](https://github.com/containers/podman/issues/8498)).
- Fixed a bug where the `podman exec` command did not move the Conmon process for the exec session into the correct cgroup.
- Fixed a bug where shell completion for the `ancestor` option to `podman ps --filter` did not work correctly.
- Fixed a bug where detached containers would not properly clean themselves up (or remove themselves if `--rm` was set) if the Podman command that created them was invoked with `--log-level=debug`.

### API
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the `Binds` and `Mounts` parameters in `HostConfig`.
- Fixed a bug where the Compat Create endpoint for Containers ignored the `Name` query parameter.
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the "default" value for `NetworkMode` (this value is used extensively by `docker-compose`) ([#8544](https://github.com/containers/podman/issues/8544)).
- Fixed a bug where the Compat Build endpoint for Images would sometimes incorrectly use the `target` query parameter as the image's tag.

### Misc
- Podman v2.2.0 vendored a non-released, custom version of the `github.com/spf13/cobra` package; this has been reverted to the latest upstream release to aid in packaging.
- Updated the containers/image library to v5.9.0

## 2.2.0
### Features
- Experimental support for shortname aliasing has been added. This is not enabled by default, but can be turned on by setting the environment variable `CONTAINERS_SHORT_NAME_ALIASING` to `on`. Documentation is [available here](https://github.com/containers/image/blob/master/docs/containers-registries.conf.5.md#short-name-aliasing).
- Initial support has been added for the `podman network connect` and `podman network disconnect` commands, which allow existing containers to modify what networks they are connected to. At present, these commands can only be used on running containers that did not specify `--network=none` when they were created.
- The `podman run` command now supports the `--network-alias` option to set network aliases (additional names the container can be accessed at from other containers via DNS if the `dnsname` CNI plugin is in use). Aliases can also be added and removed using the new `podman network connect` and `podman network disconnect` commands. Please note that this requires a new release (v1.1.0) of the `dnsname` plugin, and will only work on newly-created CNI networks.
- The `podman generate kube` command now features support for exporting container's memory and CPU limits ([#7855](https://github.com/containers/podman/issues/7855)).
- The `podman play kube` command now features support for setting CPU and Memory limits for containers ([#7742](https://github.com/containers/podman/issues/7742)).
- The `podman play kube` command now supports persistent volumes claims using Podman named volumes.
- The `podman play kube` command now supports Kubernetes configmaps via the `--configmap` option ([#7567](https://github.com/containers/podman/issues/7567)).
- The `podman play kube` command now supports a `--log-driver` option to set the log driver for created containers.
- The `podman play kube` command now supports a `--start` option, enabled by default, to start the pod after creating it. This allows for `podman play kube` to be more easily used in systemd unitfiles.
- The `podman network create` command now supports the `--ipv6` option to enable dual-stack IPv6 networking for created networks ([#7302](https://github.com/containers/podman/issues/7302)).
- The `podman inspect` command can now inspect pods, networks, and volumes, in addition to containers and images ([#6757](https://github.com/containers/podman/issues/6757)).
- The `--mount` option for `podman run` and `podman create` now supports a new type, `image`, to mount the contents of an image into the container at a given location.
- The Bash and ZSH completions have been completely reworked and have received significant enhancements! Additionally, support for Fish completions and completions for the `podman-remote` executable have been added.
- The `--log-opt` option for `podman create` and `podman run` now supports the `max-size` option to set the maximum size for a container's logs ([#7434](https://github.com/containers/podman/issues/7434)).
- The `--network` option to the `podman pod create` command now allows pods to be configured to use `slirp4netns` networking, even when run as root ([#6097](https://github.com/containers/podman/issues/6097)).
- The `podman pod stop`, `podman pod pause`, `podman pod unpause`, and `podman pod kill` commands now work on multiple containers in parallel and should be significantly faster.
- The `podman search` command now supports a `--list-tags` option to list all available tags for a single image in a single repository.
- The `podman search` command can now output JSON using the `--format=json` option.
- The `podman diff` and `podman mount` commands now work with all containers in the storage library, including those not created by Podman. This allows them to be used with Buildah and CRI-O containers.
- The `podman container exists` command now features a `--external` option to check if a container exists not just in Podman, but also in the storage library. This will allow Podman to identify Buildah and CRI-O containers.
- The `--tls-verify` and `--authfile` options have been enabled for use with remote Podman.
- The `/etc/hosts` file now includes the container's name and hostname (both pointing to localhost) when the container is run with `--net=none` ([#8095](https://github.com/containers/podman/issues/8095)).
- The `podman events` command now supports filtering events based on the labels of the container they occurred on using the `--filter label=key=value` option.
- The `podman volume ls` command now supports filtering volumes based on their labels using the `--filter label=key=value` option.
- The `--volume` and `--mount` options to `podman run` and `podman create` now support two new mount propagation options, `unbindable` and `runbindable`.
- The `name` and `id` filters for `podman pod ps` now match based on a regular expression, instead of requiring an exact match.
- The `podman pod ps` command now supports a new filter `status`, that matches pods in a certain state.

### Changes
- The `podman network rm --force` command will now also remove pods that are using the network ([#7791](https://github.com/containers/podman/issues/7791)).
- The `podman volume rm`, `podman network rm`, and `podman pod rm` commands now return exit code 1 if the object specified for removal does not exist, and exit code 2 if the object is in use and the `--force` option was not given.
- If `/dev/fuse` is passed into Podman containers as a device, Podman will open it before starting the container to ensure that the kernel module is loaded on the host and the device is usable in the container.
- Global Podman options that were not supported with remote operation have been removed from `podman-remote` (e.g. `--cgroup-manager`, `--storage-driver`).
- Many errors have been changed to remove repetition and be more clear as to what has gone wrong.
- The `--storage` option to `podman rm` is now enabled by default, with slightly changed semantics. If the given container does not exist in Podman but does exist in the storage library, it will be removed even without the `--storage` option. If the container exists in Podman it will be removed normally. The `--storage` option for `podman rm` is now deprecated and will be removed in a future release.
- The `--storage` option to `podman ps` has been renamed to `--external`. An alias has been added so the old form of the option will continue to work.
- Podman now delays the SIGTERM and SIGINT signals during container creation to ensure that Podman is not stopped midway through creating a container resulting in potential resource leakage ([#7941](https://github.com/containers/podman/issues/7941)).
- The `podman save` command now strips signatures from images it is exporting, as the formats we export to do not support signatures ([#7659](https://github.com/containers/podman/issues/7659)).
- A new `Degraded` state has been added to pods. Pods that have some, but not all, of their containers running are now considered to be `Degraded` instead of `Running`.
- Podman will now print a warning when conflicting network options related to port forwarding (e.g. `--publish` and `--net=host`) are specified when creating a container.
- The `--restart on-failure` and `--rm` options for containers no longer conflict. When both are specified, the container will be restarted if it exits with a non-zero error code, and removed if it exits cleanly ([#7906](https://github.com/containers/podman/issues/7906)).
- Remote Podman will no longer use settings from the client's `containers.conf`; defaults will instead be provided by the server's `containers.conf` ([#7657](https://github.com/containers/podman/issues/7657)).
- The `podman network rm` command now has a new alias, `podman network remove` ([#8402](https://github.com/containers/podman/issues/8402)).

### Bugfixes
- Fixed a bug where `podman load` on the remote client did not error when attempting to load a directory, which is not yet supported for remote use.
- Fixed a bug where rootless Podman could hang when the `newuidmap` binary was not installed ([#7776](https://github.com/containers/podman/issues/7776)).
- Fixed a bug where the `--pull` option to `podman run`, `podman create`,  and `podman build` did not match Docker's behavior.
- Fixed a bug where sysctl settings from the `containers.conf` configuration file were applied, even if the container did not join the namespace associated with a sysctl.
- Fixed a bug where Podman would not return the text of errors encountered when trying to run a healthcheck for a container.
- Fixed a bug where Podman was accidentally setting the `containers` environment variable in addition to the expected `container` environment variable.
- Fixed a bug where rootless Podman using CNI networking did not properly clean up DNS entries for removed containers ([#7789](https://github.com/containers/podman/issues/7789)).
- Fixed a bug where the `podman untag --all` command was not supported with remote Podman.
- Fixed a bug where the `podman system service` command could time out even if active attach connections were present ([#7826](https://github.com/containers/podman/issues/7826)).
- Fixed a bug where the `podman system service` command would sometimes never time out despite no active connections being present.
- Fixed a bug where Podman's handling of capabilities, specifically inheritable, did not match Docker's.
- Fixed a bug where `podman run` would fail if the image specified was a manifest list and had already been pulled ([#7798](https://github.com/containers/podman/pull/7798)).
- Fixed a bug where Podman did not take search registries into account when looking up images locally ([#6381](https://github.com/containers/podman/issues/6381)).
- Fixed a bug where the `podman manifest inspect` command would fail for images that had already been pulled ([#7726](https://github.com/containers/podman/issues/7726)).
- Fixed a bug where rootless Podman would not add supplemental GIDs to containers when when a user, but not a group, was set via the `--user` option to `podman create` and `podman run` and sufficient GIDs were available to add the groups ([#7782](https://github.com/containers/podman/issues/7782)).
- Fixed a bug where remote Podman commands did not properly handle cases where the user gave a name that could also be a short ID for a pod or container ([#7837](https://github.com/containers/podman/issues/7837)).
- Fixed a bug where `podman image prune` could leave images ready to be pruned after `podman image prune` was run ([#7872](https://github.com/containers/podman/issues/7872)).
- Fixed a bug where the `podman logs` command with the `journald` log driver would not read all available logs ([#7476](https://github.com/containers/podman/issues/7476)).
- Fixed a bug where the `--rm` and `--restart` options to `podman create` and `podman run` did not conflict when a restart policy that is not `on-failure` was chosen ([#7878](https://github.com/containers/podman/issues/7878)).
- Fixed a bug where the `--format "table {{ .Field }}"` option to numerous Podman commands ceased to function on Podman v2.0 and up.
- Fixed a bug where pods did not properly share an SELinux label between their containers, resulting in containers being unable to see the processes of other containers when the pod shared a PID namespace ([#7886](https://github.com/containers/podman/issues/7886)).
- Fixed a bug where the `--namespace` option to `podman ps` did not work with the remote client ([#7903](https://github.com/containers/podman/issues/7903)).
- Fixed a bug where rootless Podman incorrectly calculated the number of UIDs available in the container if multiple different ranges of UIDs were specified.
- Fixed a bug where the `/etc/hosts` file would not be correctly populated for containers in a user namespace ([#7490](https://github.com/containers/podman/issues/7490)).
- Fixed a bug where the `podman network create` and `podman network remove` commands could race when run in parallel, with unpredictable results ([#7807](https://github.com/containers/podman/issues/7807)).
- Fixed a bug where the `-p` option to `podman run`, `podman create`, and `podman pod create` would, when given only a single number (e.g. `-p 80`), assign the same port for both host and container, instead of generating a random host port ([#7947](https://github.com/containers/podman/issues/7947)).
- Fixed a bug where Podman containers did not properly store the cgroup manager they were created with, causing them to stop functioning after the cgroup manager was changed in `containers.conf` or with the `--cgroup-manager` option ([#7830](https://github.com/containers/podman/issues/7830)).
- Fixed a bug where the `podman inspect` command did not include information on the CNI networks a container was connected to if it was not running.
- Fixed a bug where the `podman attach` command would not print a newline after detaching from the container ([#7751](https://github.com/containers/podman/issues/7751)).
- Fixed a bug where the `HOME` environment variable was not set properly in containers when the `--userns=keep-id` option was set ([#8004](https://github.com/containers/podman/issues/8004)).
- Fixed a bug where the `podman container restore` command could panic when the container in question was in a pod ([#8026](https://github.com/containers/podman/issues/8026)).
- Fixed a bug where the output of the `podman image trust show --raw` command was not properly formatted.
- Fixed a bug where the `podman runlabel` command could panic if a label to run was not given ([#8038](https://github.com/containers/podman/issues/8038)).
- Fixed a bug where the `podman run` and `podman start --attach` commands would exit with an error when the user detached manually using the detach keys on remote Podman ([#7979](https://github.com/containers/podman/issues/7979)).
- Fixed a bug where rootless CNI networking did not use the `dnsname` CNI plugin if it was not available on the host, despite it always being available in the container used for rootless networking ([#8040](https://github.com/containers/podman/issues/8040)).
- Fixed a bug where Podman did not properly handle cases where an OCI runtime is specified by its full path, and could revert to using another OCI runtime with the same binary path that existed in the system `$PATH` on subsequent invocations.
- Fixed a bug where the `--net=host` option to `podman create` and `podman run` would cause the `/etc/hosts` file to be incorrectly populated ([#8054](https://github.com/containers/podman/issues/8054)).
- Fixed a bug where the `podman inspect` command did not include container network information when the container shared its network namespace (IE, joined a pod or another container's network namespace via `--net=container:...`) ([#8073](https://github.com/containers/podman/issues/8073)).
- Fixed a bug where the `podman ps` command did not include information on all ports a container was publishing.
- Fixed a bug where the `podman build` command incorrectly forwarded `STDIN` into build containers from `RUN` instructions.
- Fixed a bug where the `podman wait` command's `--interval` option did not work when units were not specified for the duration ([#8088](https://github.com/containers/podman/issues/8088)).
- Fixed a bug where the `--detach-keys` and `--detach` options could be passed to `podman create` despite having no effect (and not making sense in that context).
- Fixed a bug where Podman could not start containers if running on a system without a `/etc/resolv.conf` file (which occurs on some WSL2 images) ([#8089](https://github.com/containers/podman/issues/8089)).
- Fixed a bug where the `--extract` option to `podman cp` was nonfunctional.
- Fixed a bug where the `--cidfile` option to `podman run` would, when the container was not run with `--detach`, only create the file after the container exited ([#8091](https://github.com/containers/podman/issues/8091)).
- Fixed a bug where the `podman images` and `podman images -a` commands could panic and not list any images when certain improperly-formatted images were present in storage ([#8148](https://github.com/containers/podman/issues/8148)).
- Fixed a bug where the `podman events` command could, when the `journald` events backend was in use, become nonfunctional when a badly-formatted event or a log message that container certain string was present in the journal ([#8125](https://github.com/containers/podman/issues/8125)).
- Fixed a bug where remote Podman would, when using SSH transport, not authenticate to the server using hostkeys when connecting on a port other than 22 ([#8139](https://github.com/containers/podman/issues/8139)).
- Fixed a bug where the `podman attach` command would not exit when containers stopped ([#8154](https://github.com/containers/podman/issues/8154)).
- Fixed a bug where Podman did not properly clean paths before verifying them, resulting in Podman refusing to start if the root or temporary directories were specified with extra trailing `/` characters ([#8160](https://github.com/containers/podman/issues/8160)).
- Fixed a bug where remote Podman did not support hashed hostnames in the `known_hosts` file on the host for establishing connections ([#8159](https://github.com/containers/podman/pull/8159)).
- Fixed a bug where the `podman image exists` command would return non-zero (false) when multiple potential matches for the given name existed.
- Fixed a bug where the `podman manifest inspect` command on images that are not manifest lists would error instead of inspecting the image ([#8023](https://github.com/containers/podman/issues/8023)).
- Fixed a bug where the `podman system service` command would fail if the directory the Unix socket was to be created inside did not exist ([#8184](https://github.com/containers/podman/issues/8184)).
- Fixed a bug where pods that shared the IPC namespace (which is done by default) did not share a `/dev/shm` filesystem between all containers in the pod ([#8181](https://github.com/containers/podman/issues/8181)).
- Fixed a bug where filters passed to `podman volume list` were not inclusive ([#6765](https://github.com/containers/podman/issues/6765)).
- Fixed a bug where the `podman volume create` command would fail when the volume's data directory already existed (as might occur when a volume was not completely removed) ([#8253](https://github.com/containers/podman/issues/8253)).
- Fixed a bug where the `podman run` and `podman create` commands would deadlock when trying to create a container that mounted the same named volume at multiple locations (e.g. `podman run -v testvol:/test1 -v testvol:/test2`) ([#8221](https://github.com/containers/podman/issues/8221)).
- Fixed a bug where the parsing of the `--net` option to `podman build` was incorrect ([#8322](https://github.com/containers/podman/issues/8322)).
- Fixed a bug where the `podman build` command would print the ID of the built image twice when using remote Podman ([#8332](https://github.com/containers/podman/issues/8332)).
- Fixed a bug where the `podman stats` command did not show memory limits for containers ([#8265](https://github.com/containers/podman/issues/8265)).
- Fixed a bug where the `podman pod inspect` command printed the static MAC address of the pod in a non-human-readable format ([#8386](https://github.com/containers/podman/pull/8386)).
- Fixed a bug where the `--tls-verify` option of the `podman play kube` command had its logic inverted (`false` would enforce the use of TLS, `true` would disable it).
- Fixed a bug where the `podman network rm` command would error when trying to remove `macvlan` networks and rootless CNI networks ([#8491](https://github.com/containers/podman/issues/8491)).
- Fixed a bug where Podman was not setting sane defaults for missing `XDG_` environment variables.
- Fixed a bug where remote Podman would check if volume paths to be mounted in the container existed on the host, not the server ([#8473](https://github.com/containers/podman/issues/8473)).
- Fixed a bug where the `podman manifest create` and `podman manifest add` commands on local images would drop any images in the manifest not pulled by the host.
- Fixed a bug where networks made by `podman network create` did not include the `tuning` plugin, and as such did not support setting custom MAC addresses ([#8385](https://github.com/containers/podman/issues/8385)).
- Fixed a bug where container healthchecks did not use `$PATH` when searching for the Podman executable to run the healthcheck.
- Fixed a bug where the `--ip-range` option to `podman network create` did not properly handle non-classful subnets when calculating the last usable IP for DHCP assignment ([#8448](https://github.com/containers/podman/issues/8448)).
- Fixed a bug where the `podman container ps` alias for `podman ps` was missing ([#8445](https://github.com/containers/podman/issues/8445)).

### API
- The Compat Create endpoint for Container has received a major refactor to share more code with the Libpod Create endpoint, and should be significantly more stable.
- A Compat endpoint for exporting multiple images at once, `GET /images/get`, has been added ([#7950](https://github.com/containers/podman/issues/7950)).
- The Compat Network Connect and Network Disconnect endpoints have been added.
- Endpoints that deal with image registries now support a `X-Registry-Config` header to specify registry authentication configuration.
- The Compat Create endpoint for images now properly supports specifying images by digest.
- The Libpod Build endpoint for images now supports an `httpproxy` query parameter which, if set to true, will forward the server's HTTP proxy settings into the build container for `RUN` instructions.
- The Libpod Untag endpoint for images will now remove all tags for the given image if no repository and tag are specified for removal.
- Fixed a bug where the Ping endpoint misspelled a header name (`Libpod-Buildha-Version` instead of `Libpod-Buildah-Version`).
- Fixed a bug where the Ping endpoint sent an extra newline at the end of its response where Docker did not.
- Fixed a bug where the Compat Logs endpoint for containers did not send a newline character after each log line.
- Fixed a bug where the Compat Logs endpoint for containers would mangle line endings to change newline characters to add a preceding carriage return ([#7942](https://github.com/containers/podman/issues/7942)).
- Fixed a bug where the Compat Inspect endpoint for Containers did not properly list the container's stop signal ([#7917](https://github.com/containers/podman/issues/7917)).
- Fixed a bug where the Compat Inspect endpoint for Containers formatted the container's create time incorrectly ([#7860](https://github.com/containers/podman/issues/7860)).
- Fixed a bug where the Compat Inspect endpoint for Containers did not include the container's Path, Args, and Restart Count.
- Fixed a bug where the Compat Inspect endpoint for Containers prefixed added and dropped capabilities with `CAP_` (Docker does not do so).
- Fixed a bug where the Compat Info endpoint for the Engine did not include configured registries.
- Fixed a bug where the server could panic if a client closed a connection midway through an image pull ([#7896](https://github.com/containers/podman/issues/7896)).
- Fixed a bug where the Compat Create endpoint for volumes returned an error when a volume with the same name already existed, instead of succeeding with a 201 code ([#7740](https://github.com/containers/podman/issues/7740)).
- Fixed a bug where a client disconnecting from the Libpod or Compat events endpoints could result in the server using 100% CPU ([#7946](https://github.com/containers/podman/issues/7946)).
- Fixed a bug where the "no such image" error message sent by the Compat Inspect endpoint for Images returned a 404 status code with an error that was improperly formatted for Docker compatibility.
- Fixed a bug where the Compat Create endpoint for networks did not properly set a default for the `driver` parameter if it was not provided by the client.
- Fixed a bug where the Compat Inspect endpoint for images did not populate the `RootFS`, `VirtualSize`, `ParentId`, `Architecture`, `Os`, and `OsVersion` fields of the response.
- Fixed a bug where the Compat Inspect endpoint for images would omit the `ParentId` field if the image had no parent, and the `Created` field if the image did not have a creation time.
- Fixed a bug where the Compat Remove endpoint for Networks did not support the `Force` query parameter.

### Misc
- Updated Buildah to v1.18.0
- Updated the containers/storage library to v1.24.1
- Updated the containers/image library to v5.8.1
- Updated the containers/common library to v0.27.0

## 2.1.1
### Changes
- The `podman info` command now includes the cgroup manager Podman is using.

### Bugfixes
- Fixed a bug where Podman would not build with the `varlink` build tag enabled.
- Fixed a bug where the `podman save` command could, when asked to save multiple images, write its progress bar to the archive instead of the terminal, producing a corrupted archive.
- Fixed a bug where the `json-file` log driver did not write logs.
- Fixed a bug where `podman-remote start --attach` did not properly handle detaching using the detach keys.
- Fixed a bug where `podman pod ps --filter label=...` did not work.
- Fixed a bug where the `podman build` command did not respect the `--runtime` flag.

### API
- The REST API now includes a Server header in all responses.
- Fixed a bug where the Libpod and Compat Attach endpoints could terminate early, before sending all output from the container.
- Fixed a bug where the Compat Create endpoint for containers did not properly handle the Interactive parameter.
- Fixed a bug where the Compat Kill endpoint for containers could continue to run after a fatal error.
- Fixed a bug where the Limit parameter of the Compat List endpoint for Containers did not properly handle a limit of 0 (returning nothing, instead of all containers) ([#7722](https://github.com/containers/podman/issues/7722)).
- The Libpod Stats endpoint for containers is being deprecated and will be replaced by a similar endpoint with additional features in a future release.

## 2.1.0
### Features
- A new command, `podman image mount`, has been added. This allows for an image to be mounted, read-only, to inspect its contents without creating a container from it ([#1433](https://github.com/containers/podman/issues/1433)).
- The `podman save` and `podman load` commands can now create and load archives containing multiple images ([#2669](https://github.com/containers/podman/issues/2669)).
- Rootless Podman now supports all `podman network` commands, and rootless containers can now be joined to networks.
- The performance of `podman build` on `ADD` and `COPY` instructions has been greatly improved, especially when a `.dockerignore` is present.
- The `podman run` and `podman create` commands now support a new mode for the `--cgroups` option, `--cgroups=split`. Podman will create two cgroups under the cgroup it was launched in, one for the container and one for Conmon. This mode is useful for running Podman in a systemd unit, as it ensures that all processes are retained in systemd's cgroup hierarchy ([#6400](https://github.com/containers/podman/issues/6400)).
- The `podman run` and `podman create` commands can now specify options to slirp4netns by using the `--network` option as follows:  `--net slirp4netns:opt1,opt2`. This allows for, among other things, switching the port forwarder used by slirp4netns away from rootlessport.
- The `podman ps` command now features a new option, `--storage`, to show containers from Buildah, CRI-O and other applications.
- The `podman run` and `podman create` commands now feature a `--sdnotify` option to control the behavior of systemd's sdnotify with containers, enabling improved support for Podman in `Type=notify` units.
- The `podman run` command now features a `--preserve-fds` option to pass file descriptors from the host into the container ([#6458](https://github.com/containers/podman/issues/6458)).
- The `podman run` and `podman create` commands can now create overlay volume mounts, by adding the `:O` option to a bind mount (e.g. `-v /test:/test:O`). Overlay volume mounts will mount a directory into a container from the host and allow changes to it, but not write those changes back to the directory on the host.
- The `podman play kube` command now supports the Socket HostPath type ([#7112](https://github.com/containers/podman/issues/7112)).
- The `podman play kube` command now supports read-only mounts.
- The `podman play kube` command now supports setting labels on pods from Kubernetes metadata labels.
- The `podman play kube` command now supports setting container restart policy ([#7656](https://github.com/containers/podman/issues/7656)).
- The `podman play kube` command now properly handles `HostAlias` entries.
- The `podman generate kube` command now adds entries to `/etc/hosts` from `--host-add` generated YAML as `HostAlias` entries.
- The `podman play kube` and `podman generate kube` commands now properly support `shareProcessNamespace` to share the PID namespace in pods.
- The `podman volume ls` command now supports the `dangling` filter to identify volumes that are dangling (not attached to any container).
- The `podman run` and `podman create` commands now feature a `--umask` option to set the umask of the created container.
- The `podman create` and `podman run` commands now feature a `--tz` option to set the timezone within the container ([#5128](https://github.com/containers/podman/issues/5128)).
- Environment variables for Podman can now be added in the `containers.conf` configuration file.
- The `--mount` option of `podman run` and `podman create` now supports a new mount type, `type=devpts`, to add a `devpts` mount to the container. This is useful for containers that want to mount `/dev/` from the host into the container, but still create a terminal.
- The `--security-opt` flag to `podman run` and `podman create` now supports a new option, `proc-opts`, to specify options for the container's `/proc` filesystem.
- Podman with the `crun` OCI runtime now supports a new option to `podman run` and `podman create`, `--cgroup-conf`, which allows for advanced configuration of cgroups on cgroups v2 systems.
- The `podman create` and `podman run` commands now support a `--override-variant` option, to override the architecture variant of the image that will be pulled and ran.
- A new global option has been added to Podman, `--runtime-flags`, which allows for setting flags to use when the OCI runtime is called.
- The `podman manifest add` command now supports the `--cert-dir`, `--auth-file`, `--creds`, and `--tls-verify` options.

### Security
- This release resolves CVE-2020-14370, in which environment variables could be leaked between containers created using the Varlink API.

### Changes
- Podman will now retry pulling an image 3 times if a pull fails due to network errors.
- The `podman exec` command would previously print error messages (e.g. `exec session exited with non-zero exit code -1`) when the command run exited with a non-0 exit code. It no longer does this. The `podman exec` command will still exit with the same exit code as the command run in the container did.
- Error messages when creating a container or pod with a name that is already in use have been improved.
- For read-only containers running systemd init, Podman creates a tmpfs filesystem at `/run`. This was previously limited to 65k in size and mounted `noexec`, but is now unlimited size and mounted `exec`.
- The `podman system reset` command no longer removes configuration files for rootless Podman.

### Bugfixes
- Fixed a bug where Podman would not add an entry to `/etc/hosts` for a container if it joined another container's network namespace ([#66782](https://github.com/containers/podman/issues/6678)).
- Fixed a bug where `podman save --format oci-dir` saved the image in an incorrect format ([#6544](https://github.com/containers/podman/issues/6544)).
- Fixed a bug where privileged containers would still configure an AppArmor profile.
- Fixed a bug where the `--format` option of `podman system df` was not properly interpreting format codes that included backslashes ([#7149](https://github.com/containers/podman/issues/7149)).
- Fixed a bug where rootless Podman would ignore errors from `newuidmap` and `newgidmap`, even if `/etc/subuid` and `/etc/subgid` contained valid mappings for the user running Podman.
- Fixed a bug where the `podman commit` command did not properly handle single-character image names ([#7114](https://github.com/containers/podman/issues/7114)).
- Fixed a bug where the output of `podman ps --format=json` did not include a `Status` field ([#6980](https://github.com/containers/podman/issues/6980)).
- Fixed a bug where input to the `--log-level` option was no longer case-insensitive.
- Fixed a bug where `podman images` could segfault when an image pull was aborted while incomplete, leaving an image without a manifest ([#7444](https://github.com/containers/podman/issues/7444)).
- Fixed a bug where rootless Podman would try to create the `~/.config` directory when it did not exist, despite not placing any configuration files inside the directory.
- Fixed a bug where the output of `podman system df` was inconsistent based on whether the `-v` option was specified ([#7405](https://github.com/containers/podman/issues/7405)).
- Fixed a bug where `--security-opt apparmor=unconfined` would error if Apparmor was not enabled on the system ([#7545](https://github.com/containers/podman/issues/7545)).
- Fixed a bug where running `podman stop` on multiple containers starting with `--rm` could sometimes cause `no such container` errors ([#7384](https://github.com/containers/podman/issues/7384)).
- Fixed a bug where `podman-remote` would still try to contact the server when displaying help information about subcommands.
- Fixed a bug where the `podman build --logfile` command would segfault.
- Fixed a bug where the `podman generate systemd` command did not properly handle containers which were created with a name given as `--name=$NAME` instead of `--name $NAME` ([#7157](https://github.com/containers/podman/issues/7157)).
- Fixed a bug where the `podman ps` was ignoring the `--latest` flag.
- Fixed a bug where the `podman-remote kill` command would hang when a signal that did not kill the container was specified ([#7135](https://github.com/containers/podman/issues/7135)).
- Fixed a bug where the `--oom-score-adj` option of `podman run` and `podman create` was nonfunctional.
- Fixed a bug where the `--display` option of `podman runlabel` was nonfunctional.
- Fixed a bug where the `podman runlabel` command would not pull images that did not exist locally on the system.
- Fixed a bug where `podman-remote run` would not exit with the correct code with the container was removed by a `podman-remote rm -f` while `podman-remote run` was still running ([#7117](https://github.com/containers/podman/issues/7117)).
- Fixed a bug where the `podman-remote run --rm` command would error attempting to remove containers that had already been removed (e.g. by `podman-remote rm --force`) ([#7340](https://github.com/containers/podman/issues/7340)).
- Fixed a bug where `podman --user` with a numeric user and `podman run --userns=keepid` could create users in `/etc/passwd` in the container that belong to groups without a corresponding entry in `/etc/group` ([#7389](https://github.com/containers/podman/issues/7389)).
- Fixed a bug where `podman run --userns=keepid` could create entries in `/etc/passwd` with a UID that was already in use by another user ([#7503](https://github.com/containers/podman/issues/7503)).
- Fixed a bug where `podman --user` with a numeric user and `podman run --userns=keepid` could create users that could not be logged into ([#7499](https://github.com/containers/podman/issues/7499)).
- Fixed a bug where trying to join another container's user namespace with `--userns container:$ID` would fail ([#7547](https://github.com/containers/podman/issues/7547)).
- Fixed a bug where the `podman play kube` command would trim underscores from container names ([#7020](https://github.com/containers/podman/issues/7020)).
- Fixed a bug where the `podman attach` command would not show output when attaching to a container with a terminal ([#6523](https://github.com/containers/podman/issues/6253)).
- Fixed a bug where the `podman system df` command could be extremely slow when large quantities of images were present ([#7406](https://github.com/containers/podman/issues/7406)).
- Fixed a bug where `podman images -a` would break if any image pulled by digest was present in the store ([#7651](https://github.com/containers/podman/issues/7651)).
- Fixed a bug where the `--mount` option to `podman run` and `podman create` required the `type=` parameter to be passed first ([#7628](https://github.com/containers/podman/issues/7628)).
- Fixed a bug where the `--infra-command` parameter to `podman pod create` was nonfunctional.
- Fixed a bug where `podman auto-update` would fail for any container started with `--pull=always` ([#7407](https://github.com/containers/podman/issues/7407)).
- Fixed a bug where the `podman wait` command would only accept a single argument.
- Fixed a bug where the parsing of the `--volumes-from` option to `podman run` and `podman create` was broken, making it impossible to use multiple mount options at the same time ([#7701](https://github.com/containers/podman/issues/7701)).
- Fixed a bug where the `podman exec` command would not join executed processes to the container's supplemental groups if the container was started with both the `--user` and `--group-add` options.
- Fixed a bug where the `--iidfile` option to `podman-remote build` was nonfunctional.

### API
- The Libpod API version has been bumped to v2.0.0 due to a breaking change in the Image List API.
- Docker-compatible Volume Endpoints (Create, Inspect, List, Remove, Prune) are now available!
- Added an endpoint for generating systemd unit files for containers.
- The `last` parameter to the Libpod container list endpoint now has an alias, `limit` ([#6413](https://github.com/containers/podman/issues/6413)).
- The Libpod image list API new returns timestamps in Unix format, as integer, as opposed to as strings
- The Compat Inspect endpoint for containers now includes port information in NetworkSettings.
- The Compat List endpoint for images now features limited support for the (deprecated) `filter` query parameter ([#6797](https://github.com/containers/podman/issues/6797)).
- Fixed a bug where the Compat Create endpoint for containers was not correctly handling bind mounts.
- Fixed a bug where the Compat Create endpoint for containers would not return a 404 when the requested image was not present.
- Fixed a bug where the Compat Create endpoint for containers did not properly handle Entrypoint and Command from images.
- Fixed a bug where name history information was not properly added in the Libpod Image List endpoint.
- Fixed a bug where the Libpod image search endpoint improperly populated the Description field of responses.
- Added a `noTrunc` option to the Libpod image search endpoint.
- Fixed a bug where the Pod List API would return null, instead of an empty array, when no pods were present ([#7392](https://github.com/containers/podman/issues/7392)).
- Fixed a bug where endpoints that hijacked would do perform the hijack too early, before being ready to send and receive data ([#7195](https://github.com/containers/podman/issues/7195)).
- Fixed a bug where Pod endpoints that can operate on multiple containers at once (e.g. Kill, Pause, Unpause, Stop) would not forward errors from individual containers that failed.
- The Compat List endpoint for networks now supports filtering results ([#7462](https://github.com/containers/podman/issues/7462)).
- Fixed a bug where the Top endpoint for pods would return both a 500 and 404 when run on a nonexistent pod.
- Fixed a bug where Pull endpoints did not stream progress back to the client.
- The Version endpoints (Libpod and Compat) now provide version in a format compatible with Docker.
- All non-hijacking responses to API requests should not include headers with the version of the server.
- Fixed a bug where Libpod and Compat Events endpoints did not send response headers until the first event occurred ([#7263](https://github.com/containers/podman/issues/7263)).
- Fixed a bug where the Build endpoints (Compat and Libpod) did not stream progress to the client.
- Fixed a bug where the Stats endpoints (Compat and Libpod) did not properly handle clients disconnecting.
- Fixed a bug where the Ignore parameter to the Libpod Stop endpoint was not performing properly.
- Fixed a bug where the Compat Logs endpoint for containers did not stream its output in the correct format ([#7196](https://github.com/containers/podman/issues/7196)).

### Misc
- Updated Buildah to v1.16.1
- Updated the containers/storage library to v1.23.5
- Updated the containers/image library to v5.6.0
- Updated the containers/common library to v0.22.0

## 2.0.6
### Bugfixes
- Fixed a bug where running systemd in a container on a cgroups v1 system would fail.
- Fixed a bug where `/etc/passwd` could be re-created every time a container is restarted if the container's `/etc/passwd` did not contain an entry for the user the container was started as.
- Fixed a bug where containers without an `/etc/passwd` file specifying a non-root user would not start.
- Fixed a bug where the `--remote` flag would sometimes not make remote connections and would instead attempt to run Podman locally.

### Misc
- Updated the containers/common library to v0.14.10

## 2.0.5
### Features
- Rootless Podman will now add an entry to `/etc/passwd` for the user who ran Podman if run with `--userns=keep-id`.
- The `podman system connection` command has been reworked to support multiple connections, and re-enabled for use!
- Podman now has a new global flag, `--connection`, to specify a connection to a remote Podman API instance.

### Changes
- Podman's automatic systemd integration (activated by the `--systemd=true` flag, set by default) will now activate for containers using `/usr/local/sbin/init` as their command, instead of just `/usr/sbin/init` and `/sbin/init` (and any path ending in `systemd`).
- Seccomp profiles specified by the `--security-opt seccomp=...` flag to `podman create` and `podman run` will now be honored even if the container was created using `--privileged`.

### Bugfixes
- Fixed a bug where the `podman play kube` would not honor the `hostIP` field for port forwarding ([#5964](https://github.com/containers/podman/issues/5964)).
- Fixed a bug where the `podman generate systemd` command would panic on an invalid restart policy being specified ([#7271](https://github.com/containers/podman/issues/7271)).
- Fixed a bug where the `podman images` command could take a very long time (several minutes) to complete when a large number of images were present.
- Fixed a bug where the `podman logs` command with the `--tail` flag would not work properly when a large amount of output would be printed ([#7230](https://github.com/containers/podman/issues/7230)).
- Fixed a bug where the `podman exec` command with remote Podman would not return a non-zero exit code when the exec session failed to start (e.g. invoking a nonexistent command) ([#6893](https://github.com/containers/podman/issues/6893)).
- Fixed a bug where the `podman load` command with remote Podman would did not honor user-specified tags ([#7124](https://github.com/containers/podman/issues/7124)).
- Fixed a bug where the `podman system service` command, when run as a non-root user by Systemd, did not properly handle the Podman pause process and would not restart properly as a result ([#7180](https://github.com/containers/podman/issues/7180)).
- Fixed a bug where the `--publish` flag to `podman create`, `podman run`, and `podman pod create` did not properly handle a host IP of 0.0.0.0 (attempting to bind to literal 0.0.0.0, instead of all IPs on the system) ([#7104](https://github.com/containers/podman/issues/7014)).
- Fixed a bug where the `podman start --attach` command would not print the container's exit code when the command exited due to the container exiting.
- Fixed a bug where the `podman rm` command with remote Podman would not remove volumes, even if the `--volumes` flag was specified ([#7128](https://github.com/containers/podman/issues/7128)).
- Fixed a bug where the `podman run` command with remote Podman and the `--rm` flag could exit before the container was fully removed.
- Fixed a bug where the `--pod new:...` flag to `podman run` and `podman create` would create a pod that did not share any namespaces.
- Fixed a bug where the `--preserve-fds` flag to `podman run` and `podman exec` could close the wrong file descriptors while trying to close user-provided descriptors after passing them into the container.
- Fixed a bug where default environment variables (`$PATH` and `$TERM`) were not set in containers when not provided by the image.
- Fixed a bug where pod infra containers were not properly unmounted after exiting.
- Fixed a bug where networks created with `podman network create` with an IPv6 subnet did not properly set an IPv6 default route.
- Fixed a bug where the `podman save` command would not work properly when its output was piped to another command ([#7017](https://github.com/containers/podman/issues/7017)).
- Fixed a bug where containers using a systemd init on a cgroups v1 system could leak mounts under `/sys/fs/cgroup/systemd` to the host.
- Fixed a bug where `podman build` would not generate an event on completion ([#7022](https://github.com/containers/podman/issues/7022)).
- Fixed a bug where the `podman history` command with remote Podman printed incorrect creation times for layers ([#7122](https://github.com/containers/podman/issues/7122)).
- Fixed a bug where Podman would not create working directories specified by the container image if they did not exist.
- Fixed a bug where Podman did not clear `CMD` from the container image if the user overrode `ENTRYPOINT` ([#7115](https://github.com/containers/podman/issues/7115)).
- Fixed a bug where error parsing image names were not fully reported (part of the error message containing the exact issue was dropped).
- Fixed a bug where the `podman images` command with remote Podman did not support printing image tags in Go templates supplied to the `--format` flag ([#7123](https://github.com/containers/podman/issues/7123)).
- Fixed a bug where the `podman rmi --force` command would not attempt to unmount containers it was removing, which could cause a failure to remove the image.
- Fixed a bug where the `podman generate systemd --new` command could incorrectly quote arguments to Podman that contained whitespace, leading to nonfunctional unit files ([#7285](https://github.com/containers/podman/issues/7285)).
- Fixed a bug where the `podman version` command did not properly include build time and Git commit.
- Fixed a bug where running systemd in a Podman container on a system that did not use the `systemd` cgroup manager would fail ([#6734](https://github.com/containers/podman/issues/6734)).
- Fixed a bug where capabilities from `--cap-add` were not properly added when a container was started as a non-root user via `--user`.
- Fixed a bug where Pod infra containers were not properly cleaned up when they stopped, causing networking issues ([#7103](https://github.com/containers/podman/issues/7103)).

### API
- Fixed a bug where the libpod and compat Build endpoints did not accept the `application/tar` content type (instead only accepting `application/x-tar`) ([#7185](https://github.com/containers/podman/issues/7185)).
- Fixed a bug where the libpod Exists endpoint would attempt to write a second header in some error conditions ([#7197](https://github.com/containers/podman/issues/7197)).
- Fixed a bug where compat and libpod Network Inspect and Network Remove endpoints would return a 500 instead of 404 when the requested network was not found.
- Added a versioned `_ping` endpoint (e.g. `http://localhost/v1.40/_ping`).
- Fixed a bug where containers started through a systemd-managed instance of the REST API would be shut down when `podman system service` shut down due to its idle timeout ([#7294](https://github.com/containers/podman/issues/7294)).
- Added stronger parameter verification for the libpod Network Create endpoint to ensure subnet mask is a valid value.
- The `Pod` URL parameter to the Libpod Container List endpoint has been deprecated; the information previously gated by the `Pod` boolean will now be included in the response unconditionally.

### Misc
- Updated Buildah to v1.15.1
- Updated containers/image library to v5.5.2

## 2.0.4
### Bugfixes
- Fixed a bug where the output of `podman image search` did not populate the Description field as it was mistakenly assigned to the ID field.
- Fixed a bug where `podman build -` and `podman build` on an HTTP target would fail.
- Fixed a bug where rootless Podman would improperly chown the copied-up contents of anonymous volumes ([#7130](https://github.com/containers/podman/issues/7130)).
- Fixed a bug where Podman would sometimes HTML-escape special characters in its CLI output.
- Fixed a bug where the `podman start --attach --interactive` command would print the container ID of the container attached to when exiting ([#7068](https://github.com/containers/podman/pull/7068)).
- Fixed a bug where `podman run --ipc=host --pid=host` would only set `--pid=host` and not `--ipc=host` ([#7100](https://github.com/containers/podman/issues/7100)).
- Fixed a bug where the `--publish` argument to `podman run`, `podman create` and `podman pod create` would not allow binding the same container port to more than one host port ([#7062](https://github.com/containers/podman/issues/7062)).
- Fixed a bug where incorrect arguments to `podman images --format` could cause Podman to segfault.
- Fixed a bug where `podman rmi --force` on an image ID with more than one name and at least one container using the image would not completely remove containers using the image ([#7153](https://github.com/containers/podman/issues/7153)).
- Fixed a bug where memory usage in bytes and memory use percentage were swapped in the output of `podman stats --format=json`.

### API
- Fixed a bug where the libpod and compat events endpoints would fail if no filters were specified ([#7078](https://github.com/containers/podman/issues/7078)).
- Fixed a bug where the `CgroupVersion` field in responses from the compat Info endpoint was prefixed by "v" (instead of just being "1" or "2", as is documented).

## 2.0.3
### Features
- The `podman search` command now allows wildcards in search terms.
- The `podman play kube` command now supports the `IfNotPresent` pull type.

### Changes
- The `--disable-content-trust` flag has been added to Podman for Docker compatibility. This is a Docker-specific option and has no effect in Podman; it is provided only to ensure command line compatibility for scripts ([#7034](https://github.com/containers/podman/issues/7034)).
- Setting a static IP address or MAC address for rootless containers and pods now causes an error; previously, they were silently ignored.
- The `/sys/dev` folder is now masked in containers to prevent a potential information leak from the host.

### Bugfixes
- Fixed a bug where rootless Podman would select the wrong cgroup manager on cgroups v1 systems where the user in question had an active systemd user session ([#6982](https://github.com/containers/podman/issues/6982)).
- Fixed a bug where systems with Apparmor could not run privileged containers ([#6933](https://github.com/containers/podman/issues/6933)).
- Fixed a bug where ENTRYPOINT and CMD from images were improperly handled by `podman play kube` ([#6995](https://github.com/containers/podman/issues/6995)).
- Fixed a bug where the `--pids-limit` flag to `podman create` and `podman run` was parsed incorrectly and was unusable ([#6908](https://github.com/containers/podman/issues/6908)).
- Fixed a bug where the `podman system df` command would error if untagged images were present ([#7015](https://github.com/containers/podman/issues/7015)).
- Fixed a bug where the `podman images` command would display incorrect tags if a port number was included in the repository.
- Fixed a bug where Podman did not set a default umask and default rlimits ([#6989](https://github.com/containers/podman/issues/6989)).
- Fixed a bug where protocols in port mappings were not recognized unless they were lower-case ([#6948](https://github.com/containers/podman/issues/6948)).
- Fixed a bug where information on pod infra containers was not included in the output of `podman pod inspect`.
- Fixed a bug where Podman's systemd detection (activated by the enabled-by-default `--systemd=true` flag) would not flag a container for systemd mode if systemd was part of the entrypoint, not the command ([#6920](https://github.com/containers/podman/issues/6920)).
- Fixed a bug where `podman start --attach` was not defaulting `--sig-proxy` to true ([#6928](https://github.com/containers/podman/issues/6928)).
- Fixed a bug where `podman inspect` would show an incorrect command (`podman system service`, the command used to start the server) for containers created by a remote Podman client.
- Fixed a bug where the `podman exec` command with the remote client would not print output if the `-t` or `-i` flags where not provided.
- Fixed a bug where some variations of the `--format {{ json . }}` to `podman info` (involving added or removed whitespace) would not be accepted ([#6927](https://github.com/containers/podman/issues/6927)).
- Fixed a bug where Entrypoint could not be cleared at the command line (if unset via `--entrypoint=""`, it would be reset to the image's entrypoint) ([#6935](https://github.com/containers/podman/issues/6935)).

### API
- Fixed a bug where the events endpoints (both libpod and compat) could potentially panic on parsing filters.
- Fixed a bug where the compat Create endpoint for containers did not properly handle Entrypoint and Command.
- Fixed a bug where the Logs endpoint for containers (both libpod and compat) would not properly handle client disconnect, resulting in high CPU usage.
- The type of filters on the compat events endpoint has been adjusted to match Docker's implementation ([#6899](https://github.com/containers/podman/issues/6899)).
- The idle connection counter now properly handles hijacked connections.
- All endpoints that hijack will now properly print headers per RFC 7230 standards.

### Misc
- Updated containers/common to v0.14.6

## 2.0.2
### Changes
- The `podman system connection` command has been temporarily disabled, as it was not functioning as expected.

### Bugfixes
- Fixed a bug where the `podman ps` command would not truncate long container commands, resulting in display issues as the column could become extremely wide (the `--no-trunc` flag can be used to print the full command).
- Fixed a bug where `podman pod` commands operating on multiple containers (e.g. `podman pod stop` and `podman pod kill`) would not print errors from individual containers, but only a warning that some containers had failed.
- Fixed a bug where the `podman system service` command would panic if a connection to the Events endpoint hung up early ([#6805](https://github.com/containers/libpod/issues/6805)).
- Fixed a bug where rootless Podman would create anonymous and named volumes with the wrong owner for containers run with the `--user` directive.
- Fixed a bug where the `TMPDIR` environment variable (used for storing temporary files while pulling images) was not being defaulted (if unset) to `/var/tmp`.
- Fixed a bug where the `--publish` flag to `podman create` and `podman run` required that a host port be specified if an IP address was given ([#6806](https://github.com/containers/libpod/issues/6806)).
- Fixed a bug where in `podman-remote` commands performing an attach (`podman run`, `podman attach`, `podman start --attach`, `podman exec`) did not properly configure the terminal on Windows.
- Fixed a bug where the `--remote` flag to Podman required an argument, despite being a boolean ([#6704](https://github.com/containers/libpod/issues/6704)).
- Fixed a bug where the `podman generate systemd --new` command could generate incorrect unit files for a pod if a container in the pod was created using the `--pod=...` flag (with an =, instead of a space, before the pod ID) ([#6766](https://github.com/containers/libpod/issues/6766)).
- Fixed a bug where `NPROC` and `NOFILE` rlimits could be improperly set for rootless Podman containers, causing them to fail to start.
- Fixed a bug where `podman mount` as rootless did not error (the `podman mount` command cannot be run rootless unless it is run inside a `podman unshare` shell).
- Fixed a bug where in some cases a race in events handling code could cause error messages related to retrieving events to be lost.

### API
- Fixed a bug where the timestamp format for Libpod image list endpoint was incorrect - the format has been switched to Unix time.
- Fixed a bug where the compatibility Create endpoint did not handle empty entrypoints properly.
- Fixed a bug where the compatibility network remove endpoint would improperly handle errors where the network was not found.
- Fixed a bug where containers would be created with improper permissions because of a umask issue ([#6787](https://github.com/containers/libpod/issues/6787)).

## 2.0.1
### Changes
- The `podman system connection` command was mistakenly omitted from the 2.0 release, and has been included here.
- The `podman ps --format=json` command once again includes container's creation time in a human-readable format in the `CreatedAt` key.
- The `podman inspect` commands on containers now displays forwarded ports in a format compatible with `docker inspect`.
- The `--log-level=debug` flag to `podman run` and `podman exec` will enable syslog for exit commands, ensuring that debug logs are collected for these otherwise-unlogged commands.

### Bugfixes
- Fixed a bug where `podman build` did not properly handle the `--http-proxy` and `--cgroup-manager` flags.
- Fixed a bug where error messages related to a missing or inaccessible `/etc/subuid` or `/etc/subgid` file were very unclear ([#6572](https://github.com/containers/libpod/issues/6572)).
- Fixed a bug where the `podman logs --follow` command would not stop when the container being followed exited.
- Fixed a bug where the `--privileged` flag had mistakenly been marked as conflicting with `--group-add` and `--security-opt`.
- Fixed a bug where the `PODMAN_USERNS` environment variable was not being honored ([#6705](https://github.com/containers/libpod/issues/6705)).
- Fixed a bug where the `podman image load` command would require one argument be passed, when no arguments is also valid ([#6718](https://github.com/containers/libpod/issues/6718)).
- Fixed a bug where the bash completions did not include the `podman network` command and its subcommands.
- Fixed a bug where the mount command would not work inside of rootless containers ([#6735](https://github.com/containers/libpod/issues/6735)).
- Fixed a bug where SSH agent authentication support was not properly working in the `podman-remote` and `podman --remote` commands.
- Fixed a bug where the `podman untag` command was not erroring when no matching image was found.
- Fixed a bug where stop signal for containers was not being set properly if not explicitly provided.
- Fixed a bug where the `podman ps` command was not showing port mappings for containers which share a network namespace with another container (e.g. are part of a pod).
- Fixed a bug where the `--remote` flag could unintentionally be forwarded into containers when using `podman-remote`.
- Fixed a bug where unit files generated for pods by `podman generate systemd` would not allow individual containers to be restarted ([#6770](https://github.com/containers/libpod/issues/6770)).
- Fixed a bug where the `podman run` and `podman create` commands did not support all transports that `podman pull` does ([#6744](https://github.com/containers/libpod/issues/6744)).
- Fixed a bug where the `label` option to `--security-opt` would only be shown once in `podman inspect`, even if provided multiple times.

### API
- Fixed a bug where network endpoint URLs in the compatibility API were mistakenly suffixed with `/json`.
- Fixed a bug where the Libpod volume creation endpoint returned 200 instead of 201 on success.

### Misc
- Updated containers/common to v0.14.3

## 2.0.0
### Features
- The REST API and `podman system service` are no longer experimental, and ready for use!
- The Podman command now supports remotely connections via the REST API using the `--remote` flag.
- The Podman remote client has been entirely rewritten to use the HTTP API instead of Varlink.
- The `podman system connection` command has been added to allow configuring the endpoint that `podman-remote` and `podman --remote` will connect to.
- The `podman generate systemd` command now supports the `--new` flag when used with pods, allowing portable services for pods to be created.
- The `podman play kube` command now supports running Kubernetes Deployment YAML.
- The `podman exec` command now supports the `--detach` flag to run commands in the container in the background.
- The `-p` flag to `podman run` and `podman create` now supports forwarding ports to IPv6 addresses.
- The `podman run`, `podman create` and `podman pod create` command now support a `--replace` flag to remove and replace any existing container (or, for `pod create`, pod) with the same name
- The `--restart-policy` flag to `podman run` and `podman create` now supports the `unless-stopped` restart policy.
- The `--log-driver` flag to `podman run` and `podman create` now supports the `none` driver, which does not log the container's output.
- The `--mount` flag to `podman run` and `podman create` now accepts `readonly` option as an alias to `ro`.
- The `podman generate systemd` command now supports the `--container-prefix`, `--pod-prefix`, and `--separator` arguments to control the name of generated unit files.
- The `podman network ls` command now supports the `--filter` flag to filter results.
- The `podman auto-update` command now supports specifying an authfile to use when pulling new images on a per-container basis using the `io.containers.autoupdate.authfile` label.

### Changes
- Varlink support, including the `podman varlink` command, is deprecated and will be removed in the next release.
- As part of the implementation of the REST API, JSON output for some commands (`podman ps`, `podman images` most notably) has changed.
- Named and anonymous volumes and `tmpfs` filesystems added to containers are no longer mounted `noexec` by default.

### Bugfixes
- Fixed a bug where the `podman exec` command would log to journald when run in containers logged to journald ([#6555](https://github.com/containers/podman/issues/6555)).
- Fixed a bug where the `podman auto-update` command would not preserve the OS and architecture of the original image when pulling a replacement ([#6613](https://github.com/containers/podman/issues/6613)).
- Fixed a bug where the `podman cp` command could create an extra `merged` directory when copying into an existing directory ([#6596](https://github.com/containers/podman/issues/6596)).
- Fixed a bug where the `podman pod stats` command would crash on pods run with `--network=host` ([#5652](https://github.com/containers/podman/issues/5652)).
- Fixed a bug where containers logs written to journald did not include the name of the container.
- Fixed a bug where the `podman network inspect` and `podman network rm` commands did not properly handle non-default CNI configuration paths ([#6212](https://github.com/containers/podman/issues/6212)).
- Fixed a bug where Podman did not properly remove containers when using the Kata containers OCI runtime.
- Fixed a bug where `podman inspect` would sometimes incorrectly report the network mode of containers started with `--net=none`.
- Podman is now better able to deal with cases where `conmon` is killed before the container it is monitoring.

### Misc
- The default Podman CNI configuration now sets `HairpinMode` to allow communication between containers by connecting to a forwarded port on the host.
- Updated Buildah to v1.15.0
- Updated containers/storage to v1.20.2
- Updated containers/image to v5.5.1
- Updated containers/common to v0.14.0

## 1.9.3
### Bugfixes
- Fixed a bug where, on FIPS enabled hosts, FIPS mode secrets were not properly mounted into containers
- Fixed a bug where builds run over Varlink would hang ([#6237](https://github.com/containers/podman/issues/6237))

### Misc
- Named volumes and tmpfs filesystems will no longer default to mounting `noexec` for improved compatibility with Docker
- Updated Buildah to v1.14.9

## 1.9.2
### Bugfixes
- Fixed a bug where `podman save` would fail when the target image was specified by digest ([#5234](https://github.com/containers/podman/issues/5234))
- Fixed a bug where rootless containers with ports forwarded to them could panic and dump core due to a concurrency issue ([#6018](https://github.com/containers/podman/issues/6018))
- Fixed a bug where rootless Podman could race when opening the rootless user namespace, resulting in commands failing to run
- Fixed a bug where HTTP proxy environment variables forwarded into the container by the `--http-proxy` flag could not be overridden by `--env` or `--env-file` ([#6017](https://github.com/containers/podman/issues/6017))
- Fixed a bug where rootless Podman was setting resource limits on cgroups v2 systems that were not using systemd-managed cgroups (and thus did not support resource limits), resulting in containers failing to start

### Misc
- Rootless containers will now automatically set their ulimits to the maximum allowed for the user running the container, to match the behavior of containers run as root
- Packages managed by the core Podman team will no longer include a default `libpod.conf`, instead defaulting to `containers.conf`. The default libpod.conf will remain available in the GitHub repository until the release of Podman 2.0
- The default Podman CNI network configuration now sets HairpinMode to allow containers to access other containers via ports published on the host
- Updated containers/common to v0.8.4

## 1.9.1
### Bugfixes
- Fixed a bug where healthchecks could become nonfunctional if container log paths were manually set with `--log-path` and multiple container logs were placed in the same directory ([#5915](https://github.com/containers/podman/issues/5915))
- Fixed a bug where rootless Podman could, when using an older `libpod.conf`, print numerous warning messages about an invalid CGroup manager config
- Fixed a bug where rootless Podman would sometimes fail to close the rootless user namespace when joining it ([#5873](https://github.com/containers/podman/issues/5873))

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
- Fixed a bug where rootless Podman would print error messages about missing support for systemd cgroups when run in a container with no cgroup support ([#5488](https://github.com/containers/podman/issues/5488))
- Fixed a bug where `podman play kube` would not properly handle container-only port mappings ([#5610](https://github.com/containers/podman/issues/5610))
- Fixed a bug where the `podman container prune` command was not pruning containers in the `created` and `configured` states
- Fixed a bug where Podman was not properly removing CNI IP address allocations after a reboot ([#5433](https://github.com/containers/podman/issues/5433))
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
- Fixed a bug where `podman system reset` could delete important system directories if run as rootless on installations created by older Podman ([#4831](https://github.com/containers/podman/issues/4831))
- Fixed a bug where image built by `podman build` would not properly set the OS and Architecture they were built with ([#5503](https://github.com/containers/podman/issues/5503))
- Fixed a bug where attached `podman run` with `--sig-proxy` enabled (the default), when built with Go 1.14, would repeatedly send signal 23 to the process in the container and could generate errors when the container stopped ([#5483](https://github.com/containers/podman/issues/5483))
- Fixed a bug where rootless `podman run` commands could hang when forwarding ports
- Fixed a bug where rootless Podman would not work when `/proc` was mounted with the `hidepid` option set
- Fixed a bug where the `podman system service` command would use large amounts of CPU when `--timeout` was set to 0 ([#5531](https://github.com/containers/podman/issues/5531))

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
- The `podman run` and `podman create` commands now feature an `--rmi` flag to remove the image the container was using after it exits (if no other containers are using said image) ([#4628](https://github.com/containers/podman/issues/4628))
- The `podman create` and `podman run` commands now support the `--device-cgroup-rule` flag ([#4876](https://github.com/containers/podman/issues/4876))
- While the HTTP API remains in alpha, many fixes and additions have landed. These are documented in a separate subsection below
- The `podman create` and `podman run` commands now feature a `--no-healthcheck` flag to disable healthchecks for a container ([#5299](https://github.com/containers/podman/issues/5299))
- Containers now recognize the `io.containers.capabilities` label, which specifies a list of capabilities required by the image to run. These capabilities will be used as long as they are more restrictive than the default capabilities used
- YAML produced by the `podman generate kube` command now includes SELinux configuration passed into the container via `--security-opt label=...` ([#4950](https://github.com/containers/podman/issues/4950))

### Bugfixes
- Fixed CVE-2020-1726, a security issue where volumes manually populated before first being mounted into a container could have those contents overwritten on first being mounted into a container
- Fixed a bug where Podman containers with user namespaces in CNI networks with the DNS plugin enabled would not have the DNS plugin's nameserver added to their `resolv.conf` ([#5256](https://github.com/containers/podman/issues/5256))
- Fixed a bug where trailing `/` characters in image volume definitions could cause them to not be overridden by a user-specified mount at the same location ([#5219](https://github.com/containers/podman/issues/5219))
- Fixed a bug where the `label` option in `libpod.conf`, used to disable SELinux by default, was not being respected ([#5087](https://github.com/containers/podman/issues/5087))
- Fixed a bug where the `podman login` and `podman logout` commands required the registry to log into be specified ([#5146](https://github.com/containers/podman/issues/5146))
- Fixed a bug where detached rootless Podman containers could not forward ports ([#5167](https://github.com/containers/podman/issues/5167))
- Fixed a bug where rootless Podman could fail to run if the pause process had died
- Fixed a bug where Podman ignored labels that were specified with only a key and no value ([#3854](https://github.com/containers/podman/issues/3854))
- Fixed a bug where Podman would fail to create named volumes when the backing filesystem did not support SELinux labelling ([#5200](https://github.com/containers/podman/issues/5200))
- Fixed a bug where `--detach-keys=""` would not disable detaching from a container ([#5166](https://github.com/containers/podman/issues/5166))
- Fixed a bug where the `podman ps` command was too aggressive when filtering containers and would force `--all` on in too many situations
- Fixed a bug where the `podman play kube` command was ignoring image configuration, including volumes, working directory, labels, and stop signal ([#5174](https://github.com/containers/podman/issues/5174))
- Fixed a bug where the `Created` and `CreatedTime` fields in `podman images --format=json` were misnamed, which also broke Go template output for those fields ([#5110](https://github.com/containers/podman/issues/5110))
- Fixed a bug where rootless Podman containers with ports forwarded could hang when started ([#5182](https://github.com/containers/podman/issues/5182))
- Fixed a bug where `podman pull` could fail to parse registry names including port numbers
- Fixed a bug where Podman would incorrectly attempt to validate image OS and architecture when starting containers
- Fixed a bug where Bash completion for `podman build -f` would not list available files that could be built ([#3878](https://github.com/containers/podman/issues/3878))
- Fixed a bug where `podman commit --change` would perform incorrect validation, resulting in valid changes being rejected ([#5148](https://github.com/containers/podman/issues/5148))
- Fixed a bug where `podman logs --tail` could take large amounts of memory when the log file for a container was large ([#5131](https://github.com/containers/podman/issues/5131))
- Fixed a bug where Podman would sometimes incorrectly generate firewall rules on systems using `firewalld`
- Fixed a bug where the `podman inspect` command would not display network information for containers properly if a container joined multiple CNI networks ([#4907](https://github.com/containers/podman/issues/4907))
- Fixed a bug where the `--uts` flag to `podman create` and `podman run` would only allow specifying containers by full ID ([#5289](https://github.com/containers/podman/issues/5289))
- Fixed a bug where rootless Podman could segfault when passed a large number of file descriptors
- Fixed a bug where the `podman port` command was incorrectly interpreting additional arguments as container names, instead of port numbers
- Fixed a bug where units created by `podman generate systemd` did not depend on network targets, and so could start before the system network was ready ([#4130](https://github.com/containers/podman/issues/4130))
- Fixed a bug where exec sessions in containers which did not specify a user would not inherit supplemental groups added to the container via `--group-add`
- Fixed a bug where Podman would not respect the `$TMPDIR` environment variable for placing large temporary files during some operations (e.g. `podman pull`) ([#5411](https://github.com/containers/podman/issues/5411))

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
- The `Created` field to `podman images --format=json` has been renamed to `CreatedSince` as part of the fix for ([#5110](https://github.com/containers/podman/issues/5110)). Go templates using the old name should still work
- The `CreatedTime` field to `podman images --format=json` has been renamed to `CreatedAt` as part of the fix for ([#5110](https://github.com/containers/podman/issues/5110)). Go templates using the old name should still work
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
- Added support for using Seccomp profiles embedded in images for `podman run` and `podman create` via the new `--seccomp-policy` CLI flag ([#4806](https://github.com/containers/podman/pull/4806))
- The `podman play kube` command now honors pull policy ([#4880](https://github.com/containers/podman/issues/4880))

### Bugfixes
- Fixed a bug where the `podman cp` command would not copy the contents of directories when paths ending in `/.` were given ([#4717](https://github.com/containers/podman/issues/4717))
- Fixed a bug where the `podman play kube` command did not properly locate Seccomp profiles specified relative to localhost ([#4555](https://github.com/containers/podman/issues/4555))
- Fixed a bug where the `podman info` command for remote Podman did not show registry information ([#4793](https://github.com/containers/podman/issues/4793))
- Fixed a bug where the `podman exec` command did not support having input piped into it ([#3302](https://github.com/containers/podman/issues/3302))
- Fixed a bug where the `podman cp` command with rootless Podman on CGroups v2 systems did not properly determine if the container could be paused while copying ([#4813](https://github.com/containers/podman/issues/4813))
- Fixed a bug where the `podman container prune --force` command could possible remove running containers if they were started while the command was running ([#4844](https://github.com/containers/podman/issues/4844))
- Fixed a bug where Podman, when run as root, would not properly configure `slirp4netns` networking when requested ([#4853](https://github.com/containers/podman/pull/4853))
- Fixed a bug where `podman run --userns=keep-id` did not work when the user had a UID over 65535 ([#4838](https://github.com/containers/podman/issues/4838))
- Fixed a bug where rootless `podman run` and `podman create` with the `--userns=keep-id` option could change permissions on `/run/user/$UID` and break KDE ([#4846](https://github.com/containers/podman/issues/4846))
- Fixed a bug where rootless Podman could not be run in a systemd service on systems using CGroups v2 ([#4833](https://github.com/containers/podman/issues/4833))
- Fixed a bug where `podman inspect` would show CPUShares as 0, instead of the default (1024), when it was not explicitly set ([#4822](https://github.com/containers/podman/issues/4822))
- Fixed a bug where `podman-remote push` would segfault ([#4706](https://github.com/containers/podman/issues/4706))
- Fixed a bug where image healthchecks were not shown in the output of `podman inspect` ([#4799](https://github.com/containers/podman/issues/4799))
- Fixed a bug where named volumes created with containers from pre-1.6.3 releases of Podman would be autoremoved with their containers if the `--rm` flag was given, even if they were given names ([#5009](https://github.com/containers/podman/issues/5009))
- Fixed a bug where `podman history` was not computing image sizes correctly ([#4916](https://github.com/containers/podman/issues/4916))
- Fixed a bug where Podman would not error on invalid values to the `--sort` flag to `podman images`
- Fixed a bug where providing a name for the image made by `podman commit` was mandatory, not optional as it should be ([#5027](https://github.com/containers/podman/issues/5027))
- Fixed a bug where the remote Podman client would append an extra `"` to `%PATH` ([#4335](https://github.com/containers/podman/issues/4335))
- Fixed a bug where the `podman build` command would sometimes ignore the `-f` option and build the wrong Containerfile
- Fixed a bug where the `podman ps --filter` command would only filter running containers, instead of all containers, if `--all` was not passed ([#5050](https://github.com/containers/podman/issues/5050))
- Fixed a bug where the `podman load` command on compressed images would leave an extra copy on disk
- Fixed a bug where the `podman restart` command would not properly clean up the network, causing it to function differently from `podman stop; podman start` ([#5051](https://github.com/containers/podman/issues/5051))
- Fixed a bug where setting the `--memory-swap` flag to `podman create` and `podman run` to `-1` (to indicate unlimited) was not supported ([#5091](https://github.com/containers/podman/issues/5091))

### Misc
- Initial work on version 2 of the Podman remote API has been merged, but is still in an alpha state and not ready for use. Read more [here](https://podman.io/releases/2020/01/17/podman-new-api.html)
- Many formatting corrections have been made to the manpages
- The changes to address ([#5009](https://github.com/containers/podman/issues/5009)) may cause anonymous volumes created by Podman versions 1.6.3 to 1.7.0 to not be removed when their container is removed
- Updated vendored Buildah to v1.13.1
- Updated vendored containers/storage to v1.15.8
- Updated vendored containers/image to v5.2.0

## 1.7.0
### Features
- Added support for setting a static MAC address for containers
- Added support for creating `macvlan` networks with `podman network create`, allowing Podman containers to be attached directly to networks the host is connected to
- The `podman image prune` and `podman container prune` commands now support the `--filter` flag to filter what will be pruned, and now prompts for confirmation when run without `--force` ([#4410](https://github.com/containers/podman/issues/4410) and [#4411](https://github.com/containers/podman/issues/4411))
- Podman now creates CGroup namespaces by default on systems using CGroups v2 ([#4363](https://github.com/containers/podman/issues/4363))
- Added the `podman system reset` command to remove all Podman files and perform a factory reset of the Podman installation
- Added the `--history` flag to `podman images` to display previous names used by images ([#4566](https://github.com/containers/podman/issues/4566))
- Added the `--ignore` flag to `podman rm` and `podman stop` to not error when requested containers no longer exist
- Added the `--cidfile` flag to `podman rm` and `podman stop` to read the IDs of containers to be removed or stopped from a file
- The `podman play kube` command now honors Seccomp annotations ([#3111](https://github.com/containers/podman/issues/3111))
- The `podman play kube` command now honors `RunAsUser`, `RunAsGroup`, and `selinuxOptions`
- The output format of the `podman version` command has been changed to better match `docker version` when using the `--format` flag
- Rootless Podman will no longer initialize containers/storage twice, removing a potential deadlock preventing Podman commands from running while an image was being pulled ([#4591](https://github.com/containers/podman/issues/4591))
- Added `tmpcopyup` and `notmpcopyup` options to the `--tmpfs` and `--mount type=tmpfs` flags to `podman create` and `podman run` to control whether the content of directories are copied into tmpfs filesystems mounted over them
- Added support for disabling detaching from containers by setting empty detach keys via `--detach-keys=""`
- The `podman build` command now supports the `--pull` and `--pull-never` flags to control when images are pulled during a build
- The `podman ps -p` command now shows the name of the pod as well as its ID ([#4703](https://github.com/containers/podman/issues/4703))
- The `podman inspect` command on containers will now display the command used to create the container
- The `podman info` command now displays information on registry mirrors ([#4553](https://github.com/containers/podman/issues/4553))

### Bugfixes
- Fixed a bug where Podman would use an incorrect runtime directory as root, causing state to be deleted after root logged out and making Podman in systemd services not function properly
- Fixed a bug where the `--change` flag to `podman import` and `podman commit` was not being parsed properly in many cases
- Fixed a bug where detach keys specified in `libpod.conf` were not used by the `podman attach` and `podman exec` commands, which always used the global default `ctrl-p,ctrl-q` key combination ([#4556](https://github.com/containers/podman/issues/4556))
- Fixed a bug where rootless Podman was not able to run `podman pod stats` even on CGroups v2 enabled systems ([#4634](https://github.com/containers/podman/issues/4634))
- Fixed a bug where rootless Podman would fail on kernels without the `renameat2` syscall ([#4570](https://github.com/containers/podman/issues/4570))
- Fixed a bug where containers with chained network namespace dependencies (IE, container A using `--net container=B` and container B using `--net container=C`) would not properly mount `/etc/hosts` and `/etc/resolv.conf` into the container ([#4626](https://github.com/containers/podman/issues/4626))
- Fixed a bug where `podman run` with the `--rm` flag and without `-d` could, when run in the background, throw a 'container does not exist' error when attempting to remove the container after it exited
- Fixed a bug where named volume locks were not properly reacquired after a reboot, potentially leading to deadlocks when trying to start containers using the volume ([#4605](https://github.com/containers/podman/issues/4605) and [#4621](https://github.com/containers/podman/issues/4621))
- Fixed a bug where Podman could not completely remove containers if sent SIGKILL during removal, leaving the container name unusable without the `podman rm --storage` command to complete removal ([#3906](https://github.com/containers/podman/issues/3906))
- Fixed a bug where checkpointing containers started with `--rm` was allowed when `--export` was not specified (the container, and checkpoint, would be removed after checkpointing was complete by `--rm`) ([#3774](https://github.com/containers/podman/issues/3774))
- Fixed a bug where the `podman pod prune` command would fail if containers were present in the pods and the `--force` flag was not passed ([#4346](https://github.com/containers/podman/issues/4346))
- Fixed a bug where containers could not set a static IP or static MAC address if they joined a non-default CNI network ([#4500](https://github.com/containers/podman/issues/4500))
- Fixed a bug where `podman system renumber` would always throw an error if a container was mounted when it was run
- Fixed a bug where `podman container restore` would fail with containers using a user namespace
- Fixed a bug where rootless Podman would attempt to use the journald events backend even on systems without systemd installed
- Fixed a bug where `podman history` would sometimes not properly identify the IDs of layers in an image ([#3359](https://github.com/containers/podman/issues/3359))
- Fixed a bug where containers could not be restarted when Conmon v2.0.3 or later was used
- Fixed a bug where Podman did not check image OS and Architecture against the host when starting a container
- Fixed a bug where containers in pods did not function properly with the Kata OCI runtime ([#4353](https://github.com/containers/podman/issues/4353))
- Fixed a bug where `podman info --format '{{ json . }}' would not produce JSON output ([#4391](https://github.com/containers/podman/issues/4391))
- Fixed a bug where Podman would not verify if files passed to `--authfile` existed ([#4328](https://github.com/containers/podman/issues/4328))
- Fixed a bug where `podman images --digest` would not always print digests when they were available
- Fixed a bug where rootless `podman run` could hang due to a race with reading and writing events
- Fixed a bug where rootless Podman would print warning-level logs despite not be instructed to do so ([#4456](https://github.com/containers/podman/issues/4456))
- Fixed a bug where `podman pull` would attempt to fetch from remote registries when pulling an unqualified image using the `docker-daemon` transport ([#4434](https://github.com/containers/podman/issues/4434))
- Fixed a bug where `podman cp` would not work if STDIN was a pipe
- Fixed a bug where `podman exec` could stop accepting input if anything was typed between the command being run and the exec session starting ([#4397](https://github.com/containers/podman/issues/4397))
- Fixed a bug where `podman logs --tail 0` would print all lines of a container's logs, instead of no lines ([#4396](https://github.com/containers/podman/issues/4396))
- Fixed a bug where the timeout for `slirp4netns` was incorrectly set, resulting in an extremely long timeout ([#4344](https://github.com/containers/podman/issues/4344))
- Fixed a bug where the `podman stats` command would print CPU utilizations figures incorrectly ([#4409](https://github.com/containers/podman/issues/4409))
- Fixed a bug where the `podman inspect --size` command would not print the size of the container's read/write layer if the size was 0 ([#4744](https://github.com/containers/podman/issues/4744))
- Fixed a bug where the `podman kill` command was not properly validating signals before use ([#4746](https://github.com/containers/podman/issues/4746))
- Fixed a bug where the `--quiet` and `--format` flags to `podman ps` could not be used at the same time
- Fixed a bug where the `podman stop` command was not stopping exec sessions when a container was created without a PID namespace (`--pid=host`)
- Fixed a bug where the `podman pod rm --force` command was not removing anonymous volumes for containers that were removed
- Fixed a bug where the `podman checkpoint` command would not export all changes to the root filesystem of the container if performed more than once on the same container ([#4606](https://github.com/containers/podman/issues/4606))
- Fixed a bug where containers started with `--rm` would not be automatically removed on being stopped if an exec session was running inside the container ([#4666](https://github.com/containers/podman/issues/4666))

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
- Fixed a bug where named volumes with options did not properly detect issues with mounting the volume, leading to an inconsistent state ([#4303](https://github.com/containers/podman/issues/4303))
- Fixed a bug where incorrect Seccomp profiles were used in containers generated by `podman play kube`
- Fixed a bug where processes started by `podman exec` would have the wrong SELinux label in some circumstances ([#4361](https://github.com/containers/podman/issues/4361))
- Fixed a bug where error messages from `slirp4netns` would be lost
- Fixed a bug where `podman run --network=$NAME` would not throw an error in rootless Podman, where CNI networks are not supported
- Fixed a bug where `podman network create` would throw confusing errors when trying to create a volume with a name that already exists
- Fixed a bug where Podman would not error if the `systemd` CGroup manager was specified, but systemd could not be contacted over DBus
- Fixed a bug where image volumes were mounted `noexec` ([#4318](https://github.com/containers/podman/issues/4318))
- Fixed a bug where the `podman stats` command required the name of a container to be given, instead of showing all containers when no container was specified ([#4274](https://github.com/containers/podman/issues/4274))
- Fixed a bug where the `podman volume inspect` command would not show the options that named volumes were created with
- Fixed a bug where custom storage configuration was not written to `storage.conf` at time of first creation for rootless Podman ([#2659](https://github.com/containers/podman/issues/2659))
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
- Fixed a bug where the `podman top` command did not work on systems using CGroups V2 ([#4192](https://github.com/containers/podman/issues/4192))
- Fixed a bug where rootless Podman could double-close a file, leading to a panic
- Fixed a bug where rootless Podman could fail to retrieve some containers while refreshing the state
- Fixed a bug where `podman start --attach --sig-proxy=false` would still proxy signals into the container
- Fixed a bug where Podman would unconditionally use a non-default path for authentication credentials (`auth.json`), breaking `podman login` integration with `skopeo` and other tools using the containers/image library
- Fixed a bug where `podman ps --format=json` and `podman images --format=json` would display `null` when no results were returned, instead of valid JSON
- Fixed a bug where `podman build --squash` was incorrectly squashing all layers into one, instead of only new layers
- Fixed a bug where rootless Podman would allow volumes with options to be mounted (mounting volumes requires root), creating an inconsistent state where volumes reported as mounted but were not ([#4248](https://github.com/containers/podman/issues/4248))
- Fixed a bug where volumes which failed to unmount could not be removed ([#4247](https://github.com/containers/podman/issues/4247))
- Fixed a bug where Podman incorrectly handled some errors relating to unmounted or missing containers in containers/storage
- Fixed a bug where `podman stats` was broken on systems running CGroups V2 when run rootless ([#4268](https://github.com/containers/podman/issues/4268))
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
- Fixed a bug where rootless Podman could not correctly identify the DBus session address, causing containers to fail to start ([#4162](https://github.com/containers/podman/issues/4162))
- Fixed a bug where rootless Podman with `slirp4netns` networking would fail to start containers due to mount leaks

## 1.6.0
### Features
- The `podman network create`, `podman network rm`, `podman network inspect`, and `podman network ls` commands have been added to manage CNI networks used by Podman
- The `podman volume create` command can now create and mount volumes with options, allowing volumes backed by NFS, tmpfs, and many other filesystems
- Podman can now run containers without CGroups for better integration with systemd by using the `--cgroups=disabled` flag with `podman create` and `podman run`. This is presently only supported with the `crun` OCI runtime
- The `podman volume rm` and `podman volume inspect` commands can now refer to volumes by an unambiguous partial name, in addition to full name (e.g. `podman volume rm myvol` to remove a volume named `myvolume`) ([#3891](https://github.com/containers/podman/issues/3891))
- The `podman run` and `podman create` commands now support the `--pull` flag to allow forced re-pulling of images ([#3734](https://github.com/containers/podman/issues/3734))
- Mounting volumes into a container using `--volume`, `--mount`, and `--tmpfs` now allows the `suid`, `dev`, and `exec` mount options (the inverse of `nosuid`, `nodev`, `noexec`) ([#3819](https://github.com/containers/podman/issues/3819))
- Mounting volumes into a container using `--mount` now allows the `relabel=Z` and `relabel=z` options to relabel mounts.
- The `podman push` command now supports the `--digestfile` option to save a file containing the pushed digest
- Pods can now have their hostname set via `podman pod create --hostname` or providing Pod YAML with a hostname set to `podman play kube` ([#3732](https://github.com/containers/podman/issues/3732))
- The `podman image sign` command now supports the `--cert-dir` flag
- The `podman run` and `podman create` commands now support the `--security-opt label=filetype:$LABEL` flag to set the SELinux label for container files
- The remote Podman client now supports healthchecks

### Bugfixes
- Fixed a bug where remote `podman pull` would panic if a Varlink connection was not available ([#4013](https://github.com/containers/podman/issues/4013))
- Fixed a bug where `podman exec` would not properly set terminal size when creating a new exec session ([#3903](https://github.com/containers/podman/issues/3903))
- Fixed a bug where `podman exec` would not clean up socket symlinks on the host ([#3962](https://github.com/containers/podman/issues/3962))
- Fixed a bug where Podman could not run systemd in containers that created a CGroup namespace
- Fixed a bug where `podman prune -a` would attempt to prune images used by Buildah and CRI-O, causing errors ([#3983](https://github.com/containers/podman/issues/3983))
- Fixed a bug where improper permissions on the `~/.config` directory could cause rootless Podman to use an incorrect directory for storing some files
- Fixed a bug where the bash completions for `podman import` threw errors
- Fixed a bug where Podman volumes created with `podman volume create` would not copy the contents of their mountpoint the first time they were mounted into a container ([#3945](https://github.com/containers/podman/issues/3945))
- Fixed a bug where rootless Podman could not run `podman exec` when the container was not run inside a CGroup owned by the user ([#3937](https://github.com/containers/podman/issues/3937))
- Fixed a bug where `podman play kube` would panic when given Pod YAML without a `securityContext` ([#3956](https://github.com/containers/podman/issues/3956))
- Fixed a bug where Podman would place files incorrectly when `storage.conf` configuration items were set to the empty string ([#3952](https://github.com/containers/podman/issues/3952))
- Fixed a bug where `podman build` did not correctly inherit Podman's CGroup configuration, causing crashed on CGroups V2 systems ([#3938](https://github.com/containers/podman/issues/3938))
- Fixed a bug where `podman cp` would improperly copy files on the host when copying a symlink in the container that included a glob operator ([#3829](https://github.com/containers/podman/issues/3829))
- Fixed a bug where remote `podman run --rm` would exit before the container was completely removed, allowing race conditions when removing container resources ([#3870](https://github.com/containers/podman/issues/3870))
- Fixed a bug where rootless Podman would not properly handle changes to `/etc/subuid` and `/etc/subgid` after a container was launched
- Fixed a bug where rootless Podman could not include some devices in a container using the `--device` flag ([#3905](https://github.com/containers/podman/issues/3905))
- Fixed a bug where the `commit` Varlink API would segfault if provided incorrect arguments ([#3897](https://github.com/containers/podman/issues/3897))
- Fixed a bug where temporary files were not properly cleaned up after a build using remote Podman ([#3869](https://github.com/containers/podman/issues/3869))
- Fixed a bug where `podman remote cp` crashed instead of reporting it was not yet supported ([#3861](https://github.com/containers/podman/issues/3861))
- Fixed a bug where `podman exec` would run as the wrong user when execing into a container was started from an image with Dockerfile `USER` (or a user specified via `podman run --user`) ([#3838](https://github.com/containers/podman/issues/3838))
- Fixed a bug where images pulled using the `oci:` transport would be improperly named
- Fixed a bug where `podman varlink` would hang when managed by systemd due to SD_NOTIFY support conflicting with Varlink ([#3572](https://github.com/containers/podman/issues/3572))
- Fixed a bug where mounts to the same destination would sometimes not trigger a conflict, causing a race as to which was actually mounted
- Fixed a bug where `podman exec --preserve-fds` caused Podman to hang ([#4020](https://github.com/containers/podman/issues/4020))
- Fixed a bug where removing an unmounted container that was unmounted might sometimes not properly clean up the container ([#4033](https://github.com/containers/podman/issues/4033))
- Fixed a bug where the Varlink server would freeze when run in a systemd unit file ([#4005](https://github.com/containers/podman/issues/4005))
- Fixed a bug where Podman would not properly set the `$HOME` environment variable when the OCI runtime did not set it
- Fixed a bug where rootless Podman would incorrectly print warning messages when an OCI runtime was not found ([#4012](https://github.com/containers/podman/issues/4012))
- Fixed a bug where named volumes would conflict with, instead of overriding, `tmpfs` filesystems added by the `--read-only-tmpfs` flag to `podman create` and `podman run`
- Fixed a bug where `podman cp` would incorrectly make the target directory when copying to a symlink which pointed to a nonexistent directory ([#3894](https://github.com/containers/podman/issues/3894))
- Fixed a bug where remote Podman would incorrectly read `STDIN` when the `-i` flag was not set ([#4095](https://github.com/containers/podman/issues/4095))
- Fixed a bug where `podman play kube` would create an empty pod when given an unsupported YAML type ([#4093](https://github.com/containers/podman/issues/4093))
- Fixed a bug where `podman import --change` improperly parsed `CMD` ([#4000](https://github.com/containers/podman/issues/4000))

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
- Fixed a bug where `podman run` and `podman create` did not honor the `--authfile` option ([#3730](https://github.com/containers/podman/issues/3730))
- Fixed a bug where containers restored with `podman container restore --import` would incorrectly duplicate the Conmon PID file of the original container
- Fixed a bug where `podman build` ignored the default OCI runtime configured in `libpod.conf`
- Fixed a bug where `podman run --rm` (or force-removing any running container with `podman rm --force`) were not retrieving the correct exit code ([#3795](https://github.com/containers/podman/issues/3795))
- Fixed a bug where Podman would exit with an error if any configured hooks directory was not present
- Fixed a bug where `podman inspect` and `podman commit` would not use the correct `CMD` for containers run with `podman play kube`
- Fixed a bug created pods when using rootless Podman and CGroups V2 ([#3801](https://github.com/containers/podman/issues/3801))
- Fixed a bug where the `podman events` command with the `--since` or `--until` options could take a very long time to complete

### Misc
- Rootless Podman will now inherit OCI runtime configuration from the root configuration ([#3781](https://github.com/containers/podman/issues/3781))
- Podman now properly sets a user agent while contacting registries ([#3788](https://github.com/containers/podman/issues/3788))

## 1.5.0
### Features
- Podman containers can now join the user namespaces of other containers with `--userns=container:$ID`, or a user namespace at an arbitrary path with `--userns=ns:$PATH`
- Rootless Podman can experimentally squash all UIDs and GIDs in an image to a single UID and GID (which does not require use of the `newuidmap` and `newgidmap` executables) by passing `--storage-opt ignore_chown_errors`
- The `podman generate kube` command now produces YAML for any bind mounts the container has created ([#2303](https://github.com/containers/podman/issues/2303))
- The `podman container restore` command now features a new flag, `--ignore-static-ip`, that can be used with `--import` to import a single container with a static IP multiple times on the same host
- Added the ability for `podman events` to output JSON by specifying `--format=json`
- If the OCI runtime or `conmon` binary cannot be found at the paths specified in `libpod.conf`, Podman will now also search for them in the calling user's path
- Added the ability to use `podman import` with URLs ([#3609](https://github.com/containers/podman/issues/3609))
- The `podman ps` command now supports filtering names using regular expressions ([#3394](https://github.com/containers/podman/issues/3394))
- Rootless Podman containers with `--privileged` set will now mount in all host devices that the user can access
- The `podman create` and `podman run` commands now support the `--env-host` flag to forward all environment variables from the host into the container
- Rootless Podman now supports healthchecks ([#3523](https://github.com/containers/podman/issues/3523))
- The format of the `HostConfig` portion of the output of `podman inspect` on containers has been improved and synced with Docker
- Podman containers now support CGroup namespaces, and can create them by passing `--cgroupns=private` to `podman run` or `podman create`
- The `podman create` and `podman run` commands now support the `--ulimit=host` flag, which uses any ulimits currently set on the host for the container
- The `podman rm` and `podman rmi` commands now use different exit codes to indicate 'no such container' and 'container is running' errors
- Support for CGroups V2 through the `crun` OCI runtime has been greatly improved, allowing resource limits to be set for rootless containers when the CGroups V2 hierarchy is in use

### Bugfixes
- Fixed a bug where a race condition could cause `podman restart` to fail to start containers with ports
- Fixed a bug where containers restored from a checkpoint would not properly report the time they were started at
- Fixed a bug where `podman search` would return at most 25 results, even when the maximum number of results was set higher
- Fixed a bug where `podman play kube` would not honor capabilities set in imported YAML ([#3689](https://github.com/containers/podman/issues/3689))
- Fixed a bug where `podman run --env`, when passed a single key (to use the value from the host), would set the environment variable in the container even if it was not set on the host ([#3648](https://github.com/containers/podman/issues/3648))
- Fixed a bug where `podman commit --changes` would not properly set environment variables
- Fixed a bug where Podman could segfault while working with images with no history
- Fixed a bug where `podman volume rm` could remove arbitrary volumes if given an ambiguous name ([#3635](https://github.com/containers/podman/issues/3635))
- Fixed a bug where `podman exec` invocations leaked memory by not cleaning up files in tmpfs
- Fixed a bug where the `--dns` and `--net=container` flags to `podman run` and `podman create` were not mutually exclusive ([#3553](https://github.com/containers/podman/issues/3553))
- Fixed a bug where rootless Podman would be unable to run containers when less than 5 UIDs were available
- Fixed a bug where containers in pods could not be removed without removing the entire pod ([#3556](https://github.com/containers/podman/issues/3556))
- Fixed a bug where Podman would not properly clean up all CGroup controllers for created cgroups when using the `cgroupfs` CGroup driver
- Fixed a bug where Podman containers did not properly clean up files in tmpfs, resulting in a memory leak as containers stopped
- Fixed a bug where healthchecks from images would not use default settings for interval, retries, timeout, and start period when they were not provided by the image ([#3525](https://github.com/containers/podman/issues/3525))
- Fixed a bug where healthchecks using the `HEALTHCHECK CMD` format where not properly supported ([#3507](https://github.com/containers/podman/issues/3507))
- Fixed a bug where volume mounts using relative source paths would not be properly resolved ([#3504](https://github.com/containers/podman/issues/3504))
- Fixed a bug where `podman run` did not use authorization credentials when a custom path was specified ([#3524](https://github.com/containers/podman/issues/3524))
- Fixed a bug where containers checkpointed with `podman container checkpoint` did not properly set their finished time
- Fixed a bug where running `podman inspect` on any container not created with `podman run` or `podman create` (for example, pod infra containers) would result in a segfault ([#3500](https://github.com/containers/podman/issues/3500))
- Fixed a bug where healthcheck flags for `podman create` and `podman run` were incorrectly named ([#3455](https://github.com/containers/podman/pull/3455))
- Fixed a bug where Podman commands would fail to find targets if a partial ID was specified that was ambiguous between a container and pod ([#3487](https://github.com/containers/podman/issues/3487))
- Fixed a bug where restored containers would not have the correct SELinux label
- Fixed a bug where Varlink endpoints were not working properly if `more` was not correctly specified
- Fixed a bug where the Varlink PullImage endpoint would crash if an error occurred ([#3715](https://github.com/containers/podman/issues/3715))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not allow boolean arguments for its `ro` and `rw` options ([#2980](https://github.com/containers/podman/issues/2980))
- Fixed a bug where pods did not properly share the UTS namespace, resulting in incorrect behavior from some utilities which rely on hostname ([#3547](https://github.com/containers/podman/issues/3547))
- Fixed a bug where Podman would unconditionally append `ENTRYPOINT` to `CMD` during `podman commit` (and when reporting `CMD` in `podman inspect`) ([#3708](https://github.com/containers/podman/issues/3708))
- Fixed a bug where `podman events` with the `journald` events backend would incorrectly print 6 previous events when only new events were requested ([#3616](https://github.com/containers/podman/issues/3616))
- Fixed a bug where `podman port` would exit prematurely when a port number was specified ([#3747](https://github.com/containers/podman/issues/3747))
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
- The `cached` and `delegated` options for volume mounts are now allowed for Docker compatibility ([#3340](https://github.com/containers/podman/issues/3340))
- The `podman diff` command now supports the `--latest` flag

### Bugfixes
- Fixed a bug where `podman cp` on a single file would create a directory at the target and place the file in it ([#3384](https://github.com/containers/podman/issues/3384))
- Fixed a bug where `podman inspect --format '{{.Mounts}}'` would print a hexadecimal address instead of a container's mounts
- Fixed a bug where rootless Podman would not add an entry to container's `/etc/hosts` files for their own hostname ([#3405](https://github.com/containers/podman/issues/3405))
- Fixed a bug where `podman ps --sync` would segfault ([#3411](https://github.com/containers/podman/issues/3411))
- Fixed a bug where `podman generate kube` would produce an invalid ports configuration ([#3408](https://github.com/containers/podman/issues/3408))

### Misc
- Podman now performs much better on systems with heavy I/O load
- The `--cgroup-manager` flag to `podman` now shows the correct default setting in help if the default was overridden by `libpod.conf`
- For backwards compatibility, setting `--log-driver=json-file` in `podman run` is now supported as an alias for `--log-driver=k8s-file`. This is considered deprecated, and `json-file` will be moved to a new implementation in the future ([#3363](https://github.com/containers/podman/issues/3363))
- Podman's default `libpod.conf` file now allows the [crun](https://github.com/giuseppe/crun) OCI runtime to be used if it is installed

## 1.4.2
### Bugfixes
- Fixed a bug where Podman could not run containers using an older version of Systemd as init ([#3295](https://github.com/containers/podman/issues/3295))

### Misc
- Updated vendored Buildah to v1.9.0 to resolve a critical bug with Dockerfile `RUN` instructions
- The error message for running `podman kill` on containers that are not running has been improved
- The Podman remote client can now log to a file if syslog is not available

## 1.4.1
### Features
- The `podman exec` command now sets its error code differently based on whether the container does not exist, and the command in the container does not exist
- The `podman inspect` command on containers now outputs Mounts JSON that matches that of `docker inspect`, only including user-specified volumes and differentiating bind mounts and named volumes
- The `podman inspect` command now reports the path to a container's OCI spec with the `OCIConfigPath` key (only included when the container is initialized or running)
- The `podman run --mount` command now supports the `bind-nonrecursive` option for bind mounts ([#3314](https://github.com/containers/podman/issues/3314))

### Bugfixes
- Fixed a bug where `podman play kube` would fail to create containers due to an unspecified log driver
- Fixed a bug where Podman would fail to build with [musl libc](https://www.musl-libc.org/) ([#3284](https://github.com/containers/podman/issues/3284))
- Fixed a bug where rootless Podman using `slirp4netns` networking in an environment with no nameservers on the host other than localhost would result in nonfunctional networking ([#3277](https://github.com/containers/podman/issues/3277))
- Fixed a bug where `podman import` would not properly set environment variables, discarding their values and retaining only keys
- Fixed a bug where Podman would fail to run when built with Apparmor support but run on systems without the Apparmor kernel module loaded ([#3331](https://github.com/containers/podman/issues/3331))

### Misc
- Remote Podman will now default the username it uses to log in to remote systems to the username of the current user
- Podman now uses JSON logging with OCI runtimes that support it, allowing for better error reporting
- Updated vendored Buildah to v1.8.4
- Updated vendored containers/image to v2.0

## 1.4.0
### Features
- The `podman checkpoint` and `podman restore` commands can now be used to migrate containers between Podman installations on different systems ([#1618](https://github.com/containers/podman/issues/1618))
- The `podman cp` command now supports a `pause` flag to pause containers while copying into them
- The remote client now supports a configuration file for pre-configuring connections to remote Podman installations

### Bugfixes
- Fixed CVE-2019-10152 - The `podman cp` command improperly dereferenced symlinks in host context
- Fixed a bug where `podman commit` could improperly set environment variables that contained `=` characters ([#3132](https://github.com/containers/podman/issues/3132))
- Fixed a bug where rootless Podman would sometimes fail to start containers with forwarded ports ([#2942](https://github.com/containers/podman/issues/2942))
- Fixed a bug where `podman version` on the remote client could segfault ([#3145](https://github.com/containers/podman/issues/3145))
- Fixed a bug where `podman container runlabel` would use `/proc/self/exe` instead of the path of the Podman command when printing the command being executed
- Fixed a bug where filtering images by label did not work ([#3163](https://github.com/containers/podman/issues/3163))
- Fixed a bug where specifying a bing mount or tmpfs mount over an image volume would cause a container to be unable to start ([#3174](https://github.com/containers/podman/issues/3174))
- Fixed a bug where `podman generate kube` did not work with containers with named volumes
- Fixed a bug where rootless Podman would receive `permission denied` errors accessing `conmon.pid` ([#3187](https://github.com/containers/podman/issues/3187))
- Fixed a bug where `podman cp` with a folder specified as target would replace the folder, as opposed to copying into it ([#3184](https://github.com/containers/podman/issues/3184))
- Fixed a bug where rootless Podman commands could double-unlock a lock, causing a crash ([#3207](https://github.com/containers/podman/issues/3207))
- Fixed a bug where Podman incorrectly set `tmpcopyup` on `/dev/` mounts, causing errors when using the Kata containers runtime ([#3229](https://github.com/containers/podman/issues/3229))
- Fixed a bug where `podman exec` would fail on older kernels ([#2968](https://github.com/containers/podman/issues/2968))

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
- Fixed a bug where Podman containers with the `--rm` flag were removing created volumes when they were automatically removed ([#3071](https://github.com/containers/podman/issues/3071))
- Fixed a bug where container and pod locks were incorrectly marked as released after a system reboot, causing errors on container and pod removal ([#2900](https://github.com/containers/podman/issues/2900))
- Fixed a bug where Podman pods could not be removed if any container in the pod encountered an error during removal ([#3088](https://github.com/containers/podman/issues/3088))
- Fixed a bug where Podman pods run with the `cgroupfs` CGroup driver would encounter a race condition during removal, potentially failing to remove the pod CGroup
- Fixed a bug where the `podman container checkpoint` and `podman container restore` commands were not visible in the remote client
- Fixed a bug where `podman remote ps --ns` would not print the container's namespaces ([#2938](https://github.com/containers/podman/issues/2938))
- Fixed a bug where removing stopped containers with healthchecks could cause an error
- Fixed a bug where the default `libpod.conf` file was causing parsing errors ([#3095](https://github.com/containers/podman/issues/3095))
- Fixed a bug where pod locks were not being freed when pods were removed, potentially leading to lock exhaustion
- Fixed a bug where 'podman run' with SD_NOTIFY set could, on short-running containers, create an inconsistent state rendering the container unusable

### Misc
- The remote Podman client now uses the Varlink bridge to establish remote connections by default

## 1.3.0
### Features
- Podman now supports container restart policies! The `--restart` flag on `podman create` and `podman run` allows containers to be restarted after they exit. Please note that Podman cannot restart containers after a system reboot - for that, see our next feature
- Podman `podman generate systemd` command was added to generate systemd unit files for managing Podman containers
- The `podman runlabel` command now allows a `$GLOBAL_OPTS` variable, which will be populated by global options passed to the `podman runlabel` command, allowing custom storage configurations to be passed into containers run with `runlabel` ([#2399](https://github.com/containers/podman/issues/2399))
- The `podman play kube` command now allows `File` and `FileOrCreate` volumes
- The `podman pod prune` command was added to prune unused pods
- Added the `podman system migrate` command to migrate containers using older configurations to allow their use by newer Libpod versions ([#2935](https://github.com/containers/podman/issues/2935))
- Podman containers now forward proxy-related environment variables from the host into the container with the `--http-proxy` flag (enabled by default)
- Read-only Podman containers can now create tmpfs filesystems on `/tmp`, `/var/tmp`, and `/run` with the `--read-only-tmpfs` flag (enabled by default)
- The `podman init` command was added, performing all container pre-start tasks without starting the container to allow pre-run debugging

### Bugfixes
- Fixed a bug where `podman cp` would not copy folders ([#2836](https://github.com/containers/podman/issues/2836))
- Fixed a bug where Podman would panic when the Varlink API attempted too pull a nonexistent image ([#2860](https://github.com/containers/podman/issues/2860))
- Fixed a bug where `podman rmi` sometimes did not produce an event when images were deleted
- Fixed a bug where Podman would panic when the Varlink API passed improperly-formatted options when attempting to build ([#2869](https://github.com/containers/podman/issues/2869))
- Fixed a bug where `podman images` would not print a header if no images were present ([#2877](https://github.com/containers/podman/pull/2877))
- Fixed a bug where the `podman images` command with `--filter dangling=false` would incorrectly print dangling images instead of images which are not dangling ([#2884](https://github.com/containers/podman/issues/2884))
- Fixed a bug where rootless Podman would panic when any command was run after the system was rebooted ([#2894](https://github.com/containers/podman/issues/2894))
- Fixed a bug where Podman containers in user namespaces would include undesired directories from the host in `/sys/kernel`
- Fixed a bug where `podman create` would panic when trying to create a container whose name already existed
- Fixed a bug where `podman pull` would exit 0 on failing to pull an image ([#2785](https://github.com/containers/podman/issues/2785))
- Fixed a bug where `podman pull` would not properly print the cause of errors that occurred ([#2710](https://github.com/containers/podman/issues/2710))
- Fixed a bug where rootless Podman commands were not properly suspended via `ctrl-z` in a shell ([#2775](https://github.com/containers/podman/issues/2775))
- Fixed a bug where Podman would error when cleaning up containers when some container mountpoints in `/sys/` were cleaned up already by the closing of the mount namespace
- Fixed a bug where `podman play kube` was not including environment variables from the image run ([#2930](https://github.com/containers/podman/issues/2930))
- Fixed a bug where `podman play kube` would not properly clean up partially-created pods when encountering an error
- Fixed a bug where `podman commit` with the `--change` flag improperly set `CMD` when a multipart value was provided ([#2951](https://github.com/containers/podman/issues/2951))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not properly validate its arguments, causing Podman to panic
- Fixed a bug where conflicts between mounts created by the `--mount`, `--volume`, and `--tmpfs` flags were not properly reported
- Fixed a bug where the `--mount` flag could not be used with named volumes
- Fixed a bug where the `--mount` flag did not properly set options for created tmpfs filesystems
- Fixed a bug where rootless Podman could close too many file descriptors, causing Podman to panic ([#2964](https://github.com/containers/podman/issues/2964))
- Fixed a bug where `podman logout` would not print an error when the login was established by `docker login` ([#2735](https://github.com/containers/podman/issues/2735))
- Fixed a bug where `podman stop` would error when not all containers were running ([#2993](https://github.com/containers/podman/issues/2993))
- Fixed a bug where `podman pull` would fail to pull images by shortname if they were not present in the `docker.io` registry
- Fixed a bug where `podman login` would error when credentials were not present if a credential helper was configured ([#1675](https://github.com/containers/podman/issues/1675))
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
- The `podman logs` command can now display logs for multiple containers at the same time ([#2219](https://github.com/containers/podman/issues/2219))
- The `podman exec` command can now pass file descriptors to the process being executed in the container via the `--preserve-fds` option ([#2372](https://github.com/containers/podman/issues/2372))
- The `podman images` command can now filter images by reference ([#2266](https://github.com/containers/podman/issues/2266))
- The `podman system df` command was added to show disk usage by Podman
- The `--add-host` option can now be used by containers sharing a network namespace ([#2504](https://github.com/containers/podman/issues/2504))
- The `podman cp` command now has an `--extract` option to extract the contents of a Tar archive and copy them into the container, instead of copying the archive itself ([#2520](https://github.com/containers/podman/issues/2520))
- Podman now allows manually specifying the path of the `slirp4netns` binary for rootless networking via the `--network-cmd-path` flag ([#2506](https://github.com/containers/podman/issues/2506))
- Rootless Podman can now be used with a single UID and GID, without requiring a full 65536 UIDs/GIDs to be allocated in `/etc/subuid` and `/etc/subgid` ([#1651](https://github.com/containers/podman/issues/1651))
- The `podman runlabel` command now supports the `--replace` option to replace containers using the name requested
- Infrastructure containers for Podman pods will now attempt to use the image's `CMD` and `ENTRYPOINT` instead of a fixed command ([#2182](https://github.com/containers/podman/issues/2182))
- The `podman play kube` command now supports the `HostPath` and `VolumeMounts` YAML fields ([#2536](https://github.com/containers/podman/issues/2536))
- Added support to disable creation of `resolv.conf` or `/etc/hosts` in containers by specifying `--dns=none` and `--no-hosts`, respectively, to `podman run` and `podman create` ([#2744](https://github.com/containers/podman/issues/2744))
- The `podman version` command now supports the `{{ json . }}` template (which outputs JSON)
- Podman can now forward ports using the SCTP protocol

### Bugfixes
- Fixed a bug where directories could not be passed to `podman run --device` ([#2380](https://github.com/containers/podman/issues/2380))
- Fixed a bug where rootless Podman with the `--config` flag specified would not use appropriate defaults ([#2510](https://github.com/containers/podman/issues/2510))
- Fixed a bug where rootless Podman containers using the host network (`--net=host`) would show SELinux as enabled in the container when there were no privileges to use it
- Fixed a bug where importing very large images from `STDIN` could cause Podman to run out of memory
- Fixed a bug where some images would fail to run due to symlinks in paths where Podman would normally mount tmpfs filesystems
- Fixed a bug where `podman play kube` would sometimes segfault ([#2209](https://github.com/containers/podman/issues/2209))
- Fixed a bug where `podman runlabel` did not respect the `$PWD` variable ([#2171](https://github.com/containers/podman/issues/2171))
- Fixed a bug where error messages from refreshing the state in rootless Podman were not properly displayed ([#2584](https://github.com/containers/podman/issues/2584))
- Fixed a bug where rootless `podman build` could not access DNS servers when `slirp4netns` was in use ([#2572](https://github.com/containers/podman/issues/2572))
- Fixed a bug where rootless `podman stop` and `podman rm` would not work on containers which specified a non-root user ([#2577](https://github.com/containers/podman/issues/2577))
- Fixed a bug where container labels whose values contained commas were incorrectly parsed and caused errors creating containers ([#2574](https://github.com/containers/podman/issues/2574))
- Fixed a bug where calling Podman with a nonexistent command would exit 0, instead of with an appropriate error code ([#2530](https://github.com/containers/podman/issues/2530))
- Fixed a bug where rootless `podman exec` would fail when `--user` was specified ([#2566](https://github.com/containers/podman/issues/2566))
- Fixed a bug where, when a container had a name that was a fragment of another container's ID, Podman would refuse to operate on the first container by name
- Fixed a bug where `podman pod create` would fail if a pod shared no namespaces but created an infra container
- Fixed a bug where rootless Podman failed on the S390 and CRIS architectures
- Fixed a bug where `podman rm` would exit 0 if no containers specified were found ([#2539](https://github.com/containers/podman/issues/2539))
- Fixed a bug where `podman run` would fail to enable networking for containers with additional CNI networks specified ([#2795](https://github.com/containers/podman/issues/2795))
- Fixed a bug where the `podman images` command on the remote client was not displaying digests ([#2756](https://github.com/containers/podman/issues/2756))
- Fixed a bug where Podman was unable to clean up mounts in containers using user namespaces
- Fixed a bug where `podman image save` would, when told to save to a path that exists, return an error, but still delete the file at the given path
- Fixed a bug where specifying environment variables containing commas with `--env` would cause parsing errors ([#2712](https://github.com/containers/podman/issues/2712))
- Fixed a bug where `podman umount` would not error if called with no arguments
- Fixed a bug where the user and environment variables specified by the image used in containers created by `podman create kube` was being ignored ([#2665](https://github.com/containers/podman/issues/2665))
- Fixed a bug where the `podman pod inspect` command would segfault if not given an argument ([#2681](https://github.com/containers/podman/issues/2681))
- Fixed a bug where rootless `podman pod top` would fail ([#2682](https://github.com/containers/podman/issues/2682))
- Fixed a bug where the `podman load` command would not error if an input file is not specified and a file was not redirected to `STDIN`
- Fixed a bug where rootless `podman` could fail if global configuration was altered via flag (for example, `--root`, `--runroot`, `--storage-driver`)
- Fixed a bug where forwarded ports that were part of a range (e.g. 20-30) were displayed individually by `podman ps`, as opposed to together as a range ([#1358](https://github.com/containers/podman/issues/1358))
- Fixed a bug where `podman run --rootfs` could panic ([#2654](https://github.com/containers/podman/issues/2654))
- Fixed a bug where `podman build` would fail if options were specified after the directory to build ([#2636](https://github.com/containers/podman/issues/2636))
- Fixed a bug where image volumes made by `podman create` and `podman run` would have incorrect permissions ([#2634](https://github.com/containers/podman/issues/2634))
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
- Fixed a bug where running Podman with the `--config` flag would not set an appropriate default value for `tmp_dir` ([#2408](https://github.com/containers/podman/issues/2408))
- Fixed a bug where the `podman logs` command with the `--timestamps` flag produced unreadable output ([#2500](https://github.com/containers/podman/issues/2500))
- Fixed a bug where the `podman cp` command would automatically extract `.tar` files copied into the container ([#2509](https://github.com/containers/podman/issues/2509))

### Misc
- The `podman container stop` command is now usable with the Podman remote client

## 1.1.1
### Bugfixes
- Fixed a bug where `podman container restore` was erroneously available as `podman restore` ([#2191](https://github.com/containers/podman/issues/2191))
- Fixed a bug where the `volume_path` option in `libpod.conf` was not being respected
- Fixed a bug where Podman failed to build when the `varlink` tag was not present ([#2459](https://github.com/containers/podman/issues/2459))
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
- Fixed a bug where `podman build` could not build a container from images tagged locally that did not exist in a registry ([#2469](https://github.com/containers/podman/issues/2469))
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
- Rootless Podman will now pull some configuration options (for example, OCI runtime path) from the default root `libpod.conf` if they are not explicitly set in the user's own `libpod.conf` ([#2174](https://github.com/containers/podman/issues/2174))
- Added an alias `-f` for the `--format` flag of the `podman info` and `podman version` commands
- Added an alias `-s` for the `--size` flag of the `podman inspect` command
- Added the `podman system info` and `podman system prune` commands
- Added the `podman cp` command to copy files between containers and the host ([#613](https://github.com/containers/podman/issues/613))
- Added the `--password-stdin` flag to `podman login`
- Added the `--all-tags` flag to `podman pull`
- The `--rm` and `--detach` flags can now be used together with `podman run`
- The `podman start` and `podman run` commands for containers in pods will now start dependency containers if they are stopped
- Added the `podman system renumber` command to handle lock changes
- The `--net=host` and `--dns` flags for `podman run` and `podman create` no longer conflict
- Podman now handles mounting the shared /etc/resolv.conf from network namespaces created by `ip netns add` when they are passed in via `podman run --net=ns:`

### Bugfixes
- Fixed a bug with `podman inspect` where different information would be returned when the container was running versus when it was stopped
- Fixed a bug where errors in Go templates passed to `podman inspect` were silently ignored instead of reported to the user ([#2159](https://github.com/containers/podman/issues/2159))
- Fixed a bug where rootless Podman with `--pid=host` containers was incorrectly masking paths in `/proc`
- Fixed a bug where full errors starting rootless `Podman` were not reported when a refresh was requested
- Fixed a bug where Podman would override the config file-specified storage driver with the driver the backing database was created with without warning users
- Fixed a bug where `podman prune` would prune all images not in use by a container, as opposed to only untagged images, by default ([#2192](https://github.com/containers/podman/issues/2192))
- Fixed a bug where `podman create --quiet` and `podman run --quiet` were not properly suppressing output
- Fixed a bug where the `table` keyword in Go template output of `podman ps` was not working ([#2221](https://github.com/containers/podman/issues/2221))
- Fixed a bug where `podman inspect` on images pulled by digest would double-print `@sha256` in output when printing digests ([#2086](https://github.com/containers/podman/issues/2086))
- Fixed a bug where `podman container runlabel` will return a non-0 exit code if the label does not exist
- Fixed a bug where container state was always reset to Created after a reboot ([#1703](https://github.com/containers/podman/issues/1703))
- Fixed a bug where `/dev/pts` was unconditionally overridden in rootless Podman, which was unnecessary except in very specific cases
- Fixed a bug where Podman run as root was ignoring some options in `/etc/containers/storage.conf` ([#2217](https://github.com/containers/podman/issues/2217))
- Fixed a bug where Podman cleanup processes were not being given the proper OCI runtime path if a custom one was specified
- Fixed a bug where `podman images --filter dangling=true` would crash if no dangling images were present ([#2246](https://github.com/containers/podman/issues/2246))
- Fixed a bug where `podman ps --format "{{.Mounts}}"` would not display a container's mounts ([#2238](https://github.com/containers/podman/issues/2238))
- Fixed a bug where `podman pod stats` was ignoring Go templates specified by `--format` ([#2258](https://github.com/containers/podman/issues/2258))
- Fixed a bug where `podman generate kube` would fail on containers with `--user` specified ([#2304](https://github.com/containers/podman/issues/2304))
- Fixed a bug where `podman images` displayed incorrect output for images pulled by digest ([#2175](https://github.com/containers/podman/issues/2175))
- Fixed a bug where `podman port` and `podman ps` did not properly display ports if the container joined a network namespace from a pod or another container ([#846](https://github.com/containers/podman/issues/846))
- Fixed a bug where detaching from a container using the detach keys would cause Podman to hang until the container exited
- Fixed a bug where `podman create --rm` did not work with `podman start --attach`
- Fixed a bug where invalid named volumes specified in `podman create` and `podman run` could cause segfaults ([#2301](https://github.com/containers/podman/issues/2301))
- Fixed a bug where the `runtime` field in `libpod.conf` was being ignored. `runtime` is legacy and deprecated, but will continue to be respected for the foreseeable future
- Fixed a bug where `podman login` would sometimes report it logged in successfully when it did not
- Fixed a bug where `podman pod create` would not error on receiving unused CLI argument
- Fixed a bug where rootless `podman run` with the `--pod` argument would fail if the pod was stopped
- Fixed a bug where `podman images` did not print a trailing newline when not invoked on a TTY ([#2388](https://github.com/containers/podman/issues/2388))
- Fixed a bug where the `--runtime` option was sometimes not overriding `libpod.conf`
- Fixed a bug where `podman pull` and `podman runlabel` would sometimes exit with 0 when they should have exited with an error ([#2405](https://github.com/containers/podman/issues/2405))
- Fixed a bug where rootless `podman export -o` would fail ([#2381](https://github.com/containers/podman/issues/2381))
- Fixed a bug where read-only volumes would fail in rootless Podman when the volume originated on a filesystem mounted `nosuid`, `nodev`, or `noexec` ([#2312](https://github.com/containers/podman/issues/2312))
- Fixed a bug where some files used by checkpoint and restore received improper SELinux labels ([#2334](https://github.com/containers/podman/issues/2334))
- Fixed a bug where Podman's volume path was not properly changed when containers/storage changed location ([#2395](https://github.com/containers/podman/issues/2395))

### Misc
- Podman migrated to a new, shared memory locking model in this release. As part of this, if you are running Podman with pods or dependency containers (e.g. `--net=container:`), you should run the `podman system renumber` command to migrate your containers to the new model - please reference the `podman-system-renumber(1)` man page for further details
- Podman migrated to a new command-line parsing library, and the output format of help and usage text has somewhat changed as a result
- Updated Buildah to v1.7, picking up a number of bugfixes
- Updated containers/image library to v1.5, picking up a number of bugfixes and performance improvements to pushing images
- Updated containers/storage library to v1.10, picking up a number of bugfixes
- Work on the remote Podman client for interacting with Podman remotely over Varlink is progressing steadily, and many image and pod commands are supported - please see the [Readme](https://github.com/containers/podman/blob/main/remote_client.md) for details
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
