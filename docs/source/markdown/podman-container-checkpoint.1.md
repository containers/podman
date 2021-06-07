% podman-container-checkpoint(1)

## NAME
podman\-container\-checkpoint - Checkpoints one or more running containers

## SYNOPSIS
**podman container checkpoint** [*options*] *container* ...

## DESCRIPTION
Checkpoints all the processes in one or more containers. You may use container IDs or names as input.

## OPTIONS
#### **--all**, **-a**

Checkpoint all running containers.

#### **--compress**, **-c**

Specify the compression algorithm used for the checkpoint archive created
with the **--export, -e** option. Possible algorithms are *gzip*, *none*
and *zstd*. If no compression algorithm is specified Podman will use
*zstd*.

One possible reason to use *none* is to enable faster creation of checkpoint
archives. Not compressing the checkpoint archive can result in faster checkpoint
archive creation.

```
# podman container checkpoint -l --compress=none --export=dump.tar
# podman container checkpoint -l --compress=gzip --export=dump.tar.gz
```

#### **--export**, **-e**

Export the checkpoint to a tar.gz file. The exported checkpoint can be used
to import the container on another system and thus enabling container live
migration. This checkpoint archive also includes all changes to the container's
root file-system, if not explicitly disabled using **--ignore-rootfs**

#### **--ignore-rootfs**

This only works in combination with **--export, -e**. If a checkpoint is
exported to a tar.gz file it is possible with the help of **--ignore-rootfs**
to explicitly disable including changes to the root file-system into
the checkpoint archive file.

#### **--ignore-volumes**

This option must be used in combination with the **--export, -e** option.
When this option is specified, the content of volumes associated with
the container will not be included into the checkpoint tar.gz file.

#### **--keep**, **-k**

Keep all temporary log and statistics files created by CRIU during checkpointing. These files
are not deleted if checkpointing fails for further debugging. If checkpointing succeeds these
files are theoretically not needed, but if these files are needed Podman can keep the files
for further analysis.

#### **--latest**, **-l**

Instead of providing the container name or ID, checkpoint the last created container. (This option is not available with the remote Podman client)

#### **--leave-running**, **-R**

Leave the container running after checkpointing instead of stopping it.

#### **--pre-checkpoint**, **-P**

Dump the container's memory information only, leaving the container running. Later
operations will supersede prior dumps. It only works on runc 1.0-rc3 or higher.

#### **--tcp-established**

Checkpoint a container with established TCP connections. If the checkpoint
image contains established TCP connections, this options is required during
restore. Defaults to not checkpointing containers with established TCP
connections.

#### **--with-previous**

Check out the container with previous criu image files in pre-dump. It only works
without **--pre-checkpoint** or **-P**. It only works on runc 1.0-rc3 or higher.

## EXAMPLE

podman container checkpoint mywebserver

podman container checkpoint 860a4b23

podman container checkpoint -P -e pre-checkpoint.tar.gz -l

podman container checkpoint --with-previous -e checkpoint.tar.gz -l

## SEE ALSO
podman(1), podman-container-restore(1)

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
