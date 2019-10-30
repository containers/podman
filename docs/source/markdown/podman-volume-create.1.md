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

**--driver**=*driver*

Specify the volume driver name (default local).

**--help**

Print usage statement

**-l**, **-label**=*label*

Set metadata for a volume (e.g., --label mykey=value).

**-o**, **--opt**=*option*

Set driver specific options.
For the default driver, `local`, this allows a volume to be configured to mount a filesystem on the host.
For the `local` driver the following options are supported: `type`, `device`, and `o`.
The `type` option sets the type of the filesystem to be mounted, and is equivalent to the `-t` flag to **mount(8)**.
The `device` option sets the device to be mounted, and is equivalent to the `device` argument to **mount(8)**.
The `o` option sets options for the mount, and is equivalent to the `-o` flag to **mount(8)** with two exceptions.
The `o` option supports `uid` and `gid` options to set the UID and GID of the created volume that are not normally supported by **mount(8)**.
Using volume options with the `local` driver requires root privileges.

## EXAMPLES

```
$ podman volume create myvol

$ podman volume create

$ podman volume create --label foo=bar myvol

# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=nodev,noexec myvol

# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=uid=1000,gid=1000 testvol
```

## SEE ALSO
podman-volume(1), mount(8)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
