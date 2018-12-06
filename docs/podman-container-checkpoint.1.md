% podman-container-checkpoint(1)

## NAME
podman\-container\-checkpoint - Checkpoints one or more running containers

## SYNOPSIS
**podman container checkpoint** [*options*] *container* ...

## DESCRIPTION
Checkpoints all the processes in one or more containers. You may use container IDs or names as input.

## OPTIONS
**-k**, **--keep**

Keep all temporary log and statistics files created by CRIU during checkpointing. These files
are not deleted if checkpointing fails for further debugging. If checkpointing succeeds these
files are theoretically not needed, but if these files are needed Podman can keep the files
for further analysis.

**--all, -a**

Checkpoint all running containers.

**--latest, -l**

Instead of providing the container name or ID, checkpoint the last created container.

**--leave-running, -R**

Leave the container running after checkpointing instead of stopping it.

**--tcp-established**

Checkpoint a container with established TCP connections. If the checkpoint
image contains established TCP connections, this options is required during
restore. Defaults to not checkpointing containers with established TCP
connections.

## EXAMPLE

podman container checkpoint mywebserver

podman container checkpoint 860a4b23

## SEE ALSO
podman(1), podman-container-restore(1)

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
