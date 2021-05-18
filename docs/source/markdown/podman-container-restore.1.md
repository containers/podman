% podman-container-restore(1)

## NAME
podman\-container\-restore - Restores one or more containers from a checkpoint

## SYNOPSIS
**podman container restore** [*options*] *container* ...

## DESCRIPTION
Restores a container from a checkpoint. You may use container IDs or names as input.

## OPTIONS
#### **--keep**, **-k**

Keep all temporary log and statistics files created by CRIU during
checkpointing as well as restoring. These files are not deleted if restoring
fails for further debugging. If restoring succeeds these files are
theoretically not needed, but if these files are needed Podman can keep the
files for further analysis. This includes the checkpoint directory with all
files created during checkpointing. The size required by the checkpoint
directory is roughly the same as the amount of memory required by the
processes in the checkpointed container.

Without the **-k**, **--keep** option the checkpoint will be consumed and cannot be used
again.

#### **--all**, **-a**

Restore all checkpointed containers.

#### **--latest**, **-l**

Instead of providing the container name or ID, restore the last created container. (This option is not available with the remote Podman client)

#### **--tcp-established**

Restore a container with established TCP connections. If the checkpoint image
contains established TCP connections, this option is required during restore.
If the checkpoint image does not contain established TCP connections this
option is ignored. Defaults to not restoring containers with established TCP
connections.

#### **--import**, **-i**

Import a checkpoint tar.gz file, which was exported by Podman. This can be used
to import a checkpointed container from another host. Do not specify a *container*
argument when using this option.

#### **--import-previous**

Import a pre-checkpoint tar.gz file which was exported by Podman. This option
must be used with **-i** or **--import**. It only works on runc 1.0-rc3 or higher.

#### **--name**, **-n**

This is only available in combination with **--import, -i**. If a container is restored
from a checkpoint tar.gz file it is possible to rename it with **--name, -n**. This
way it is possible to restore a container from a checkpoint multiple times with different
names.

If the **--name, -n** option is used, Podman will not attempt to assign the same IP
address to the container it was using before checkpointing as each IP address can only
be used once and the restored container will have another IP address. This also means
that **--name, -n** cannot be used in combination with **--tcp-established**.

#### **--ignore-rootfs**

This is only available in combination with **--import, -i**. If a container is restored
from a checkpoint tar.gz file it is possible that it also contains all root file-system
changes. With **--ignore-rootfs** it is possible to explicitly disable applying these
root file-system changes to the restored container.

#### **--ignore-static-ip**

If the container was started with **--ip** the restored container also tries to use that
IP address and restore fails if that IP address is already in use. This can happen, if
a container is restored multiple times from an exported checkpoint with **--name, -n**.

Using **--ignore-static-ip** tells Podman to ignore the IP address if it was configured
with **--ip** during container creation.

#### **--ignore-static-mac**

If the container was started with **--mac-address** the restored container also
tries to use that MAC address and restore fails if that MAC address is already
in use. This can happen, if a container is restored multiple times from an
exported checkpoint with **--name, -n**.

Using **--ignore-static-mac** tells Podman to ignore the MAC address if it was
configured with **--mac-address** during container creation.

#### **--ignore-volumes**

This option must be used in combination with the **--import, -i** option.
When restoring containers from a checkpoint tar.gz file with this option,
the content of associated volumes will not be restored.

#### **--publish**, **-p**

Replaces the ports that the container publishes, as configured during the
initial container start, with a new set of port forwarding rules.

```
# podman run --rm -p 2345:80 -d webserver
# podman container checkpoint -l --export=dump.tar
# podman container restore -p 5432:8080 --import=dump.tar
```

For more details please see **podman run --publish**.

## EXAMPLE

podman container restore mywebserver

podman container restore 860a4b23

podman container restore --import-previous pre-checkpoint.tar.gz --import checkpoint.tar.gz

## SEE ALSO
podman(1), podman-container-checkpoint(1), podman-run(1)

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
