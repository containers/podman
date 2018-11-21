% podman-container-restore(1)

## NAME
podman\-container\-restore - Restores one or more running containers

## SYNOPSIS
**podman container restore** [*options*] *container* ...

## DESCRIPTION
Restores a container from a checkpoint. You may use container IDs or names as input.

## OPTIONS
**-k**, **--keep**

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

**--all, -a**

Restore all checkpointed containers.

**--latest, -l**

Instead of providing the container name or ID, restore the last created container.

## EXAMPLE

podman container restore mywebserver

podman container restore 860a4b23

## SEE ALSO
podman(1), podman-container-checkpoint(1)

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
