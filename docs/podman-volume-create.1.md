% podman-volume-create(1)

## NAME
podman\-volume\-create - Create a new volume

## SYNOPSIS
**podman volume create** [*options*]

## DESCRIPTION

Creates an empty volume and prepares it to be used by containers. The volume
can be created with a specific name, if a name is not given a random name is
generated. You can add metadata to the volume by using the **--label** flag and
driver options can be set using the **--opt** flag.

## OPTIONS

**--driver**=""

Specify the volume driver name (default local).

**--help**

Print usage statement

**-l**, **--label**=[]

Set metadata for a volume (e.g., --label mykey=value).

**-o**, **--opt**=[]

Set driver specific options. To setup NFS volume you need to specify:

    type: `-o type=nfs` To indicate the nfs mount.

    o: `-o o=addr=nfsserver.example.com,rw` Options including the address of the nfs server.

    device: `-o device=/nfsshare`, the remote nfs share.

## EXAMPLES

```
$ podman volume create myvol

$ podman volume create

$ podman volume create --label foo=bar myvol

# podman volume create --opt type=nfs --opt o=addr=192.168.0.2,rw --opt device=/nfsshare  mynfsvol
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
