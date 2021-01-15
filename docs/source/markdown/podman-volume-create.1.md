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

#### **--driver**=*driver*

Specify the volume driver name (default **local**). Setting this to a value other than **local** Podman will attempt to create the volume using a volume plugin with the given name. Such plugins must be defined in the **volume_plugins** section of the **containers.conf**(5) configuration file.

#### **--help**

Print usage statement

#### **--label**=*label*, **-l**

Set metadata for a volume (e.g., --label mykey=value).

#### **--opt**=*option*, **-o**

Set driver specific options.
For the default driver, **local**, this allows a volume to be configured to mount a filesystem on the host.
For the `local` driver the following options are supported: `type`, `device`, and `o`.
The `type` option sets the type of the filesystem to be mounted, and is equivalent to the `-t` flag to **mount(8)**.
The `device` option sets the device to be mounted, and is equivalent to the `device` argument to **mount(8)**.
The `o` option sets options for the mount, and is equivalent to the `-o` flag to **mount(8)** with two exceptions.
The `o` option supports `uid` and `gid` options to set the UID and GID of the created volume that are not normally supported by **mount(8)**.
Using volume options with the **local** driver requires root privileges.
When not using the **local** driver, the given options will be passed directly to the volume plugin. In this case, supported options will be dictated by the plugin in question, not Podman.

## EXAMPLES

```
$ podman volume create myvol

$ podman volume create

$ podman volume create --label foo=bar myvol

# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=nodev,noexec myvol

# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=uid=1000,gid=1000 testvol
```

## SEE ALSO
**podman-volume**(1), **mount**(8), **containers.conf**(5)

## HISTORY
January 2020, updated with information on volume plugins by Matthew Heon <mheon@redhat.com>
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
