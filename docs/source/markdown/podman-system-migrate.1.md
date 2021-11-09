% podman-system-migrate(1)

## NAME
podman\-system\-migrate - Migrate existing containers to a new podman version

## SYNOPSIS
**podman system migrate** [*options*]

## DESCRIPTION
**podman system migrate** migrates containers to the latest podman version.

**podman system migrate** takes care of migrating existing containers to the latest version of podman if any change is necessary.

"Rootless Podman uses a pause process to keep the unprivileged
namespaces alive. This prevents any change to the `/etc/subuid` and
`/etc/subgid` files from being propagated to the rootless containers
while the pause process is running.

For these changes to be propagated, it is necessary to first stop all
running containers associated with the user and to also stop the pause
process and delete its pid file.  Instead of doing it manually, `podman
system migrate` can be used to stop both the running containers and the
pause process. The `/etc/subuid` and `/etc/subgid` files can then be
edited or changed with usermod to recreate the user namespace with the
newly configured mappings.

## OPTIONS

#### **--new-runtime**=*runtime*

Set a new OCI runtime for all containers.
This can be used after a system upgrade which changes the default OCI runtime to move all containers to the new runtime.
There are no guarantees that the containers will continue to work under the new runtime, as some runtimes support differing options and configurations.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **usermod(8)**

## HISTORY
April 2019, Originally compiled by Giuseppe Scrivano (gscrivan at redhat dot com)
