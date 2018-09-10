# Release Notes

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

### Compatability:
We switched JSON encoding/decoding to a new library for this release to address a compatability issue introduced by v0.8.2.
However, this may cause issues with containers created in 0.8.2 and 0.8.3 with custom DNS servers.
