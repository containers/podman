% podman-container-checkpoint(1)

## NAME
podman\-container\-checkpoint - Checkpoints one or more running containers

## SYNOPSIS
**podman container checkpoint** [*options*] *container* ...

## DESCRIPTION
Checkpoints all the processes in one or more containers. You may use container IDs or names as input.

## OPTIONS
**--keep**, **-k**

Keep all temporary log and statistics files created by CRIU during checkpointing. These files
are not deleted if checkpointing fails for further debugging. If checkpointing succeeds these
files are theoretically not needed, but if these files are needed Podman can keep the files
for further analysis.

**--all**, **-a**

Checkpoint all running containers.

**--latest**, **-l**

Instead of providing the container name or ID, checkpoint the last created container.

The latest option is not supported on the remote client.

**--leave-running**, **-R**

Leave the container running after checkpointing instead of stopping it.

**--tcp-established**

Checkpoint a container with established TCP connections. If the checkpoint
image contains established TCP connections, this options is required during
restore. Defaults to not checkpointing containers with established TCP
connections.

**--export, -e**

Export the checkpoint to a tar.gz file. The exported checkpoint can be used
to import the container on another system and thus enabling container live
migration. This checkpoint archive also includes all changes to the container's
root file-system, if not explicitly disabled using **--ignore-rootfs**

**--ignore-rootfs**

This only works in combination with **--export, -e**. If a checkpoint is
exported to a tar.gz file it is possible with the help of **--ignore-rootfs**
to explicitly disable including changes to the root file-system into
the checkpoint archive file.

## EXAMPLE

podman container checkpoint mywebserver

podman container checkpoint 860a4b23

## SEE ALSO
podman(1), podman-container-restore(1)

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
