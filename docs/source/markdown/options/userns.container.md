####> This option file is used in:
####>   podman create, kube play, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--userns**=*mode*

Set the user namespace mode for the container. It defaults to the **PODMAN_USERNS** environment variable. An empty value ("") means user namespaces are disabled unless an explicit mapping is set with the **--uidmap** and **--gidmap** options.

This option is incompatible with **--gidmap**, **--uidmap**, **--subuidname** and **--subgidname**.

Rootless user --userns=Key mappings:

Key       | Host User |  Container User
----------|---------------|---------------------
""        |$UID         |0 (Default User account mapped to root user in container.)
keep-id   |$UID         |$UID (Map user account to same UID within container.)
keep-id:uid=200,gid=210 |$UID| 200:210 (Map user account to specified uid, gid value within container.)
auto      |$UID         | nil (Host User UID is not mapped into container.)
nomap     |$UID         | nil (Host User UID is not mapped into container.)

Valid _mode_ values are:

**auto**[:_OPTIONS,..._]: automatically create a unique user namespace.

The `--userns=auto` flag, requires that the user name `containers` and a range of subordinate user ids that the Podman container is allowed to use be specified in the /etc/subuid and /etc/subgid files.

Example: `containers:2147483647:2147483648`.

Podman allocates unique ranges of UIDs and GIDs from the `containers` subordinate user ids. The size of the ranges is based on the number of UIDs required in the image. The number of UIDs and GIDs can be overridden with the `size` option.

The rootless option `--userns=keep-id` uses all the subuids and subgids of the user. Using `--userns=auto` when starting new containers will not work as long as any containers exist that were started with `--userns=keep-id`.

  Valid `auto` options:

  - *gidmapping*=_CONTAINER\_GID:HOST\_GID:SIZE_: to force a GID mapping to be present in the user namespace.
  - *size*=_SIZE_: to specify an explicit size for the automatic user namespace. e.g. `--userns=auto:size=8192`. If `size` is not specified, `auto` will estimate a size for the user namespace.
  - *uidmapping*=_CONTAINER\_UID:HOST\_UID:SIZE_: to force a UID mapping to be present in the user namespace.

**container:**_id_: join the user namespace of the specified container.

**host**: run in the user namespace of the caller. The processes running in the container will have the same privileges on the host as any other process launched by the calling user (default).

**keep-id**: creates a user namespace where the current rootless user's UID:GID are mapped to the same values in the container. This option is not allowed for containers created by the root user.

  Valid `keep-id` options:

  - *uid*=UID: override the UID inside the container that will be used to map the current rootless user to.
  - *gid*=GID: override the GID inside the container that will be used to map the current rootless user to.

**nomap**: creates a user namespace where the current rootless user's UID:GID are not mapped into the container. This option is not allowed for containers created by the root user.

**ns:**_namespace_: run the <<container|pod>> in the given existing user namespace.
